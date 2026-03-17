package go_pool

import "sync"

type ITask interface {
	Run(executor ...any) any
}
type IGoPool interface {
	Start()
	Schedule(task ITask)
	WaitAndClose()
}
type goPool struct {
	executors []any
	workers   int
	tasks     chan ITask
	wg        sync.WaitGroup
}

func NewPool(workers int, executors ...any) IGoPool {
	if workers <= 0 {
		workers = 1
	}
	if len(executors) != 0 {
		workers = len(executors)
	}
	p := &goPool{
		workers:   workers,
		tasks:     make(chan ITask, workers*2),
		executors: executors,
	}
	return p
}
func (p *goPool) Start() {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		if len(p.executors) > 0 {
			go p.worker(p.executors[i])
		} else {
			go p.worker()
		}
	}
}

func (p *goPool) worker(executor ...any) {
	defer p.wg.Done()
	for task := range p.tasks {
		if len(executor) > 0 {
			task.Run(executor[0])
		} else {
			task.Run()
		}
	}
}
func (p *goPool) Schedule(task ITask) {
	p.tasks <- task
}

func (p *goPool) WaitAndClose() {
	close(p.tasks)
	p.wg.Wait()
}
