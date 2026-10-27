package mission

import (
	"context"
	"fmt"
	"log"
	"mygin-api/internal/config"
	"mygin-api/internal/model"
	"mygin-api/internal/pool"
	"runtime/debug"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/panjf2000/ants/v2"
	"github.com/redis/go-redis/v9"
)

const orderTimeoutKey = "order:timeout"

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
	o.appCfg.Logger.Info("All orders processed and pool released.")
}

func (o *OrderTimeout) Run(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			// 增加调用栈打印，便于定位线上问题
			o.appCfg.Logger.Error("Recovered in OrderTimeout.Run: %v\nStack:\n%s", r, debug.Stack())
		}
	}()

	pool, err := pool.NewWorkpool(10, o.appCfg.Logger)
	if err != nil {
		o.appCfg.Logger.Error("Failed to create workpool: %v", err)
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
			o.appCfg.Logger.Info("OrderTimeout context done")
			return
		case <-ticker.C:
			o.SubmitOrder(ctx, pool)
		}
	}

}

func (o *OrderTimeout) SubmitOrder(ctx context.Context, pool *pool.Workpool) error {
	//处理订单超时逻辑
	now := float64(time.Now().UnixNano())

	// ZRANGEBYSCORE key min max LIMIT offset count
	// 获取所有 score <= now 的成员（即已超时的订单）
	// 注意：这里为了简单演示，一次性获取最多 10 个过期订单
	orders, err := o.appCfg.RedisClient.ZRangeByScore(ctx, orderTimeoutKey, &redis.ZRangeBy{
		Min:    "-inf",
		Max:    fmt.Sprintf("%f", now),
		Offset: 0,
		Count:  10,
	}).Result()
	if err != nil {
		o.appCfg.Logger.Error("Failed to get orders from redis: %v", err)
		return err
	}

	if len(orders) == 0 {
		return nil
	}

	for _, orderNo := range orders {
		currentOrderNo := orderNo
		// 尝试从 Sorted Set 中移除该成员，确保只被处理一次
		// 使用 ZREM 的返回值判断是否成功移除，防止并发重复处理
		_, err := o.appCfg.RedisClient.ZRem(ctx, orderTimeoutKey, currentOrderNo).Result()
		if err != nil {
			o.appCfg.Logger.Error("Failed to remove order from redis: %v", err)
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
						o.appCfg.Logger.Error("[Error] Final failure for order %s: %v", orderNo, err)
						return err
					}
					return nil
				},
				retry.Context(ctx),                // 支持上下文取消
				retry.Attempts(3),                 // 最多重试3次
				retry.Delay(500*time.Millisecond), // 基础延迟
				retry.MaxDelay(2*time.Second),     // 最大延迟
				retry.OnRetry(func(n uint, err error) {
					log.Printf("[Retry] Order %s attempt %d failed: %v", orderNo, n+1, err)
				}),
				retry.RetryIf(func(err error) bool {
					// 仅对特定错误进行重试，例如网络超时或临时服务器错误
					// 这里简单模拟：如果错误包含 "timeout" 则重试
					return true
				}),
			)
			if err != nil {
				o.appCfg.Logger.Error("Failed to process order %s: %v", orderNo, err)
				return
			}

		})
		o.wg.Wait()
		if err != nil {
			o.wg.Done() // 提交失败时必须手动递减，防止 WaitGroup 死锁
			o.appCfg.Logger.Error("Failed to submit order %s to pool: %v", orderNo, err)
			// 提交失败，将订单重新加回队列，防止数据丢失
			o.appCfg.RedisClient.ZAdd(ctx, orderTimeoutKey, redis.Z{
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
	result := o.appCfg.DB.Table("orders").
		Where("order_no = ?", orderNo).
		First(&order)
	//判断订单是否存在
	if result.Error != nil {
		return result.Error

	}
	//判断订单是否已支付或已完成
	if order.Status == 1 || order.Status == 3 {
		return fmt.Errorf("order %s cannot be cancelled due to current status: %d", orderNo, order.Status)
	}
	//更新订单已取消
	result = o.appCfg.DB.Table("orders").
		Where("order_no = ?", orderNo).
		Where("version = ?", order.Version). //版本号 乐观锁
		Update("status", 4)
	if result.Error != nil {
		return fmt.Errorf("update order %s status failed: %w", orderNo, result.Error)
	}

	// 处理乐观锁冲突：影响行数为0说明在此期间订单已被其他进程修改
	if result.RowsAffected == 0 {
		return fmt.Errorf("update order %s conflict: version mismatch", orderNo)
	}
	return nil
}
