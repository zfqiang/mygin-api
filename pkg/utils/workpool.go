package utils

import (
	"log/slog"
	"time"

	"github.com/panjf2000/ants/v2"
)

// Workpool is a work pool
type Workpool struct {
	GlobalPool *ants.Pool
}

// 创建一个工作池，指定工作池大小，并返回一个 Workpool 对象
func NewWorkpool(size int, expiryDuration time.Duration, logger *slog.Logger) (*Workpool, error) {

	// WithNonblocking: 默认行为 (false)‌：‌阻塞模式‌,‌当工作池已满时，Submit 会阻塞直到有空闲的 worker 可用。
	// WithNonblocking(true): 非阻塞模式，当工作池已满时，Submit 会立即返回一个 error。
	//建议‌：高并发突发流量场景建议开启，并配合上层重试或降级逻辑。
	// WithPreAlloc: 默认行为 (false)‌：‌不预先分配 worker，当提交任务时才会创建 worker。
	//‌开启 (true)‌：适合超大容量池（如数万级别）且任务执行时间较长的场景。可以减少运行时内存分配的 GC 开销。‌关闭 (false)‌：默认值。适合容量较小或任务波动大的场景，节省初始内存占用。
	// WithExpiryDuration: worker 空闲超过此时间会被回收
	// WithPanicHandler: 捕获任务中的 panic，防止协程崩溃导致服务不可用
	pool, err := ants.NewPool(size,
		ants.WithNonblocking(false),
		ants.WithPreAlloc(false),
		ants.WithExpiryDuration(expiryDuration),
		ants.WithPanicHandler(func(p interface{}) {
			logger.Error("Task panicked", "panicked", p)
		}))
	if err != nil {
		logger.Error("Failed to create ants pool:", "error", err)
		return nil, err

	}
	logger.Info("Ants pool initialized", "size", size, "expiryDuration", expiryDuration)
	workPool := &Workpool{
		GlobalPool: pool,
	}
	return workPool, nil
}

func (p *Workpool) Close() {
	p.GlobalPool.Release()
}

func (p *Workpool) Submit(task func()) error {
	return p.GlobalPool.Submit(task)
}
