// internal/pkg/async/pool.go
package async

import (
	"context"
	"sync"
)

type Task struct {
	Name    string
	Execute func() (interface{}, error)
}

type Result struct {
	Name string
	Data interface{}
	Err  error
}

type Pool struct {
	workerCount int
	tasks       chan Task
	results     chan Result
}

func NewPool(workerCount int) *Pool {
	return &Pool{
		workerCount: workerCount,
		tasks:       make(chan Task),
		results:     make(chan Result),
	}
}

func (p *Pool) worker(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case task, ok := <-p.tasks:
			if !ok {
				return
			}
			data, err := task.Execute()
			p.results <- Result{
				Name: task.Name,
				Data: data,
				Err:  err,
			}
		case <-ctx.Done():
			return
		}
	}
}

func (p *Pool) Execute(ctx context.Context, tasks []Task) map[string]Result {
	var wg sync.WaitGroup
	results := make(map[string]Result)

	// Start workers
	for i := 0; i < p.workerCount; i++ {
		wg.Add(1)
		go p.worker(ctx, &wg)
	}

	// Send tasks
	go func() {
		for _, task := range tasks {
			select {
			case p.tasks <- task:
			case <-ctx.Done():
				return
			}
		}
		close(p.tasks)
	}()

	// Collect results
	for i := 0; i < len(tasks); i++ {
		select {
		case result := <-p.results:
			results[result.Name] = result
		case <-ctx.Done():
			return results
		}
	}

	wg.Wait()
	close(p.results)

	return results
}
