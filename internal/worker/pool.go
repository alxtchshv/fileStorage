package worker

import (
	"context"
	"log/slog"
	"sync"
)

// Job — задача для пула.
type Job struct {
	ID string
	Fn func(ctx context.Context) error
}

// Pool — пул горутин для конкурентного выполнения задач.
// chan Job — безопасная очередь задач между горутинами.
// range chan — читает задачи пока канал открыт, завершается при close().
// sync.WaitGroup — ждать завершения всех воркеров при Stop.
// sync.Once — close(jobs) вызывается ровно один раз даже при параллельных Stop().
// Backpressure: Submit блокируется если канал полный — замедляем приём файлов.
type Pool struct {
	jobs chan Job
	wg   sync.WaitGroup
	size int
	once sync.Once
}

func NewPool(size int) *Pool {
	p := &Pool{
		jobs: make(chan Job, size*10),
		size: size,
	}
	for i := range size {
		p.wg.Add(1)
		go p.worker(i)
	}
	slog.Info("воркер пул запущен", "workers", size)
	return p
}

func (p *Pool) Submit(job Job) {
	p.jobs <- job
}

// Stop ждёт завершения текущих задач, потом останавливает воркеры.
func (p *Pool) Stop() {
	p.once.Do(func() {
		close(p.jobs)
		p.wg.Wait()
		slog.Info("воркер пул остановлен")
	})
}

func (p *Pool) worker(id int) {
	defer p.wg.Done()
	for job := range p.jobs {
		slog.Debug("воркер берёт задачу", "worker", id, "job", job.ID)
		if err := job.Fn(context.Background()); err != nil {
			slog.Error("ошибка задачи", "worker", id, "job", job.ID, "err", err)
		}
	}
}
