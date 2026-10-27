package pool

import (
	"log"
	"log/slog"
	"time"

	"github.com/panjf2000/ants/v2"
)

// Workpool is a work pool
type Workpool struct {
	GlobalPool *ants.Pool
}

// 创建一个工作池，指定工作池大小，并返回一个 Workpool 对象
func NewWorkpool(size int, logger *slog.Logger) (*Workpool, error) {

	// WithExpiryDuration: worker 空闲超过此时间会被回收
	// WithPanicHandler: 捕获任务中的 panic，防止协程崩溃导致服务不可用
	pool, err := ants.NewPool(size,
		ants.WithExpiryDuration(10*time.Second),
		ants.WithPanicHandler(func(p interface{}) {
			logger.Error("Task panicked: %v", p)
		}))
	if err != nil {
		logger.Error("Failed to create ants pool:", err)
		return nil, err

	}
	log.Printf("Ants pool initialized with size: %d", size)
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
