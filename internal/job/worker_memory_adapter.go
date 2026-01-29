package job

import "context"

// MemoryWorker adapts the in-process Queue to the common Worker interface.
type MemoryWorker struct {
	q *Queue
}

func NewMemoryWorker(q *Queue) *MemoryWorker {
	return &MemoryWorker{q: q}
}

func (w *MemoryWorker) Submit(ctx context.Context, job *Job) error { return w.q.Submit(ctx, job) }

func (w *MemoryWorker) Start(ctx context.Context) error {
	w.q.Start(ctx)
	return nil
}

func (w *MemoryWorker) Stop() { w.q.Stop() }


