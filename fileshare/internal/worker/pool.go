// Package worker
package worker

import (
	"context"
	"sync"
	"time"
)

type Job interface {
	Process()
}

type Pool struct {
	JobQueue chan Job
	WorkerCount int
	wg sync.WaitGroup
}

func NewPool(workerCount int, queueSize int) *Pool {
	return &Pool {
		JobQueue: make(chan Job, queueSize),
		WorkerCount: workerCount,
	}
}

func (p *Pool) Start() {
	for i := 0; i < p.WorkerCount; i++ {
		p.wg.Add(1)
		go func(workerID int) {
			defer p.wg.Done()
			for job := range p.JobQueue {
				job.Process()
			}
		}(i)
	}
}

func (p *Pool) TrySubmit(ctx context.Context, job Job) bool {
	select {
	case p.JobQueue <- job:
		return true
	case <- ctx.Done():
		return false
	}
}

func (p *Pool) SubmitWithTimeout(job Job, timeout time.Duration) bool {
	select {
	case p.JobQueue <- job:
		return true
	case <- time.After(timeout):
		return false
	}
}

func (p *Pool) QueueLength() int {
	return len(p.JobQueue)
}

func (p *Pool) QueueCapacity() int {
	return cap(p.JobQueue)
}

func (p *Pool) Stop() {
	close(p.JobQueue)
	p.wg.Wait()
}
