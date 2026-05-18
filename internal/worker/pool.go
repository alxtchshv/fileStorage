package worker

import (
	"context"
	"sync"
)

// Job — задача для пула.
type Job struct {
	ID string
	Fn func(ctx context.Context) error
}

// Pool — пул горутин для конкурентного выполнения задач.
//
// Ключевые концепции:
//   - chan Job (буферизованный) — очередь задач между горутинами
//   - range chan — читать задачи пока канал не закрыт (close)
//   - sync.WaitGroup — ждать завершения всех воркеров при Stop
//   - sync.Once — close(jobs) вызывается ровно один раз, даже при параллельных Stop()
//   - backpressure: Submit блокируется если канал полный — сервер замедляет приём файлов
type Pool struct {
	jobs chan Job
	wg   sync.WaitGroup
	size int
	once sync.Once
}

// NewPool создаёт пул и запускает size воркеров.
// Буфер канала = size*10: принимать задачи без блокировки пока воркеры заняты.
// Запуск воркеров: for i := range size { wg.Add(1); go p.worker(i) }
func NewPool(size int) *Pool {
	return &Pool{
		jobs: make(chan Job, size*10),
		size: size,
	}
}

// Submit кладёт задачу в канал.
// Если канал полный — блокируется (backpressure).
func (p *Pool) Submit(job Job) {
}

// Stop останавливает пул после завершения текущих задач.
// close(p.jobs) сигнализирует воркерам завершить range.
// p.wg.Wait() ждёт пока все воркеры завершат текущие задачи.
func (p *Pool) Stop() {
	p.once.Do(func() {
	})
}

// worker — горутина-воркер. Запускается в NewPool.
// range p.jobs блокируется в ожидании задачи и завершается когда канал закрыт.
// При ошибке задачи логируй и продолжай — один файл не должен убивать воркер.
func (p *Pool) worker(id int) {
	defer p.wg.Done()
}
