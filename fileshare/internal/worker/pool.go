// Package worker
package worker

import (
	"sync"
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

func (p *Pool) Submit(job Job) {
	p.JobQueue <- job
}

func (p *Pool) Stop() {
	close(p.JobQueue)
	p.wg.Wait()
}
