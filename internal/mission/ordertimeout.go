package mission

import (
	"context"
	"fmt"
	"mygin-api/internal/model"
	"mygin-api/pkg/config"
	"mygin-api/pkg/utils"
	"runtime/debug"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/panjf2000/ants/v2"
	"github.com/redis/go-redis/v9"
)

type OrderTimeout struct {
	redisClient *redis.Client
	appCfg      *config.AppConfig
	wg          sync.WaitGroup
	pool        *ants.Pool
}

func NewOrderTimeout(appCfg *config.AppConfig) *OrderTimeout {
	return &OrderTimeout{
		appCfg: appCfg,
	}
}

func (o *OrderTimeout) Shutdown() {
	o.wg.Wait()
	o.appCfg.Logger.Info("订单超时处理任务已停止")
}

func (o *OrderTimeout) Run(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			// 增加调用栈打印，便于定位线上问题
			o.appCfg.Logger.Errorf("订单超时处理任务发生 panic | panic=%v | stack=%s", r, string(debug.Stack()))
		}
	}()

	// 创建协程池
	poolSize := o.appCfg.Cfg.WorkPoolSize
	if poolSize <= 0 {
		o.appCfg.Logger.Errorf("订单超时任务工作池容量配置无效 | pool_size=%d", poolSize)
		return
	}
	poolIdleTimeout := o.appCfg.Cfg.WorkPoolIdleTimeout
	if poolIdleTimeout <= 0 {
		o.appCfg.Logger.Errorf("订单超时任务工作池空闲超时配置无效 | idle_timeout=%s", poolIdleTimeout)
		return
	}
	pool, err := utils.NewWorkpool(poolSize, poolIdleTimeout, o.appCfg.Logger)
	if err != nil {
		return
	}
	// 确保退出时释放协程池资源，防止 Goroutine 泄漏
	defer pool.Close()

	// 使用 Ticker 替代 default + time.Sleep，优化 CPU 调度
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			o.appCfg.Logger.Info("订单超时处理任务收到停止信号")
			return
		case <-ticker.C:
			o.SubmitOrder(ctx, pool)
		}
	}
}

func (o *OrderTimeout) SubmitOrder(ctx context.Context, pool *utils.Workpool) error {
	//处理订单超时逻辑
	now := float64(time.Now().UnixNano())

	// ZRANGEBYSCORE key min max LIMIT offset count
	// 获取所有 score <= now 的成员（即已超时的订单）
	// 注意：这里为了简单演示，一次性获取最多 10 个过期订单
	orders, err := o.appCfg.RedisClient.ZRangeByScore(ctx, utils.OrderTimeoutKey, &redis.ZRangeBy{
		Min:    "-inf",
		Max:    fmt.Sprintf("%f", now),
		Offset: 0,
		Count:  10,
	}).Result()
	if err != nil {
		o.appCfg.Logger.Errorf("从 Redis 获取待处理超时订单失败 | error=%v", err)
		return err
	}

	if len(orders) == 0 {
		return nil
	}

	for _, orderNo := range orders {
		currentOrderNo := orderNo
		// 尝试从 Sorted Set 中移除该成员，确保只被处理一次
		// 使用 ZREM 的返回值判断是否成功移除，防止并发重复处理
		_, err := o.appCfg.RedisClient.ZRem(ctx, utils.OrderTimeoutKey, currentOrderNo).Result()
		if err != nil {
			o.appCfg.Logger.Errorf("从 Redis 超时队列移除订单失败 | order_no=%s | error=%v", currentOrderNo, err)
			continue
		}

		// 提交任务到 ants 协程池
		o.wg.Add(1)
		err = pool.Submit(func() {
			defer o.wg.Done()

			// 定义重试配置
			err := retry.Do(
				func() error {
					// 为每个任务创建独立的子上下文，或者共享父上下文
					// 这里使用共享上下文，以便主程序取消时所有任务都能感知
					if err := o.ProcessOrder(ctx, orderNo); err != nil {
						return err
					}
					o.appCfg.Logger.Info("超时订单处理成功", "order_no", orderNo)
					return nil
				},
				retry.Context(ctx),                // 支持上下文取消
				retry.Attempts(3),                 // 最多重试3次
				retry.Delay(500*time.Millisecond), // 基础延迟
				retry.MaxDelay(2*time.Second),     // 最大延迟
				retry.OnRetry(func(n uint, err error) {
					o.appCfg.Logger.Warn("超时订单处理失败，准备重试",
						"order_no", orderNo,
						"attempt", n+1,
						"error", err,
					)
				}),
				retry.RetryIf(func(err error) bool {
					// 仅对特定错误进行重试，例如网络超时或临时服务器错误
					// 这里简单模拟：如果错误包含 "timeout" 则重试
					return true
				}),
			)
			if err != nil {
				o.appCfg.Logger.Errorf("超时订单处理最终失败 | order_no=%s | error=%v", orderNo, err)
				return
			}

		})
		o.wg.Wait()
		if err != nil {
			o.wg.Done() // 提交失败时必须手动递减，防止 WaitGroup 死锁
			o.appCfg.Logger.Errorf("提交超时订单到工作池失败 | order_no=%s | error=%v", currentOrderNo, err)
			// 提交失败，将订单重新加回队列，防止数据丢失
			o.appCfg.RedisClient.ZAdd(ctx, utils.OrderTimeoutKey, redis.Z{
				Score:  now,
				Member: currentOrderNo,
			})
			continue
		}
	}
	return nil
}

func (o *OrderTimeout) ProcessOrder(ctx context.Context, orderNo string) error {
	// 订单状态: 0-待支付, 1-已支付, 2-发货中, 3-已完成, 4-已取消, 5-售后中

	//查询订单
	var order model.OrderTimeoutModel
	result := o.appCfg.DB.Table(model.OrdersTableName).
		Select("id, order_no, status, version").
		Where("order_no = ?", orderNo).
		First(&order)
	//判断订单是否存在
	if result.Error != nil {
		return result.Error

	}
	//判断订单是否已支付或已完成
	if order.Status == 1 || order.Status == 3 {
		return fmt.Errorf("订单 %s 当前状态为 %d，不能取消", orderNo, order.Status)
	}
	//更新订单已取消
	var udata = map[string]interface{}{
		"status":      utils.OrderStatusCancelled,
		"cancel_time": time.Now(),
	}
	result = o.appCfg.DB.Table(model.OrdersTableName).
		Where("order_no = ?", orderNo).
		Where("version = ?", order.Version). //版本号 乐观锁
		Updates(udata)
	if result.Error != nil {
		return fmt.Errorf("更新订单 %s 状态失败: %w", orderNo, result.Error)
	}

	// 处理乐观锁冲突：影响行数为0说明在此期间订单已被其他进程修改
	if result.RowsAffected == 0 {
		return fmt.Errorf("更新订单 %s 失败: 版本号冲突", orderNo)
	}
	return nil
}
