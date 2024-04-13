package cstream

import (
	"context"
	"sync"
)

// Stream is a simple stream of tasks with a concurrency limit.
type Stream[T any] struct {
	ctx  context.Context
	in   chan func() T
	out  chan T
	done chan struct{}
}

// NewStream creates a new StreamChan with the given context, concurrency limit, and output channel.
func NewStream[T any](ctx context.Context, c int, out chan T) *Stream[T] {
	in := make(chan func() T)
	done := make(chan struct{})
	go func() {
		defer close(done)
		stream(ctx, c, in, out)
	}()
	return &Stream[T]{
		ctx:  ctx,
		in:   in,
		out:  out,
		done: done,
	}
}

// Close stops the stream.
func (s *Stream[T]) Close() {
	close(s.in)
	close(s.done)
}

// Go sends a task to the stream's pool. All tasks are executed concurrently.
// If worker pool is full, it will block until a worker is available.
func (s *Stream[T]) Go(task func() T) {
	select {
	case <-s.done:
	case <-s.ctx.Done():
	case s.in <- task:
	}
}

// Out returns the output channel.
func (s *Stream[T]) Out() <-chan T {
	return s.out
}

// Wait blocks until stream is done or the context is canceled.
func (s *Stream[T]) Wait() error {
	select {
	case <-s.done:
		return nil
	case <-s.ctx.Done():
		return s.ctx.Err()
	}
}

// stream is a low-level function that allows you to create a stream of tasks
// with a concurrency limit.
//
// To stop the stream, close the input channel.
func stream[T any](ctx context.Context, c int, in <-chan func() T, out chan<- T) {
	if c <= 0 {
		c = 0
	}

	// Create a pool of result channels
	// to avoid allocating a new channel for each task.
	resultChPool := sync.Pool{
		New: func() any {
			return make(chan T, 1)
		},
	}

	tasks := make(chan func())
	taskResults := make(chan chan T, c)

	// Tasks Worker goroutines
	var workerWg sync.WaitGroup
	addWorker := func(initFunc func()) {
		workerWg.Add(1)
		go func() {
			defer workerWg.Done()

			if initFunc != nil {
				initFunc()
			}

			for task := range tasks {
				task()
			}
		}()
	}
	for i := 0; i < c; i++ {
		addWorker(nil)
	}

	// Result serializer goroutine
	var serializerWg sync.WaitGroup
	serializerWg.Add(1)
	go func() {
		defer serializerWg.Done()
		for resultCh := range taskResults {
			// Wait for the task to complete and get its result from the channel.
			result := <-resultCh

			// Send the result to the output channel.
			select {
			case out <- result:
			case <-ctx.Done():
				return
			}

			// Return the channel to the pool of unused channels.
			resultChPool.Put(resultCh)
		}
	}()

	// Feed the workers with tasks
	for inTask := range in {
		// Skip nil tasks to prevent panics.
		if inTask == nil {
			continue
		}

		// Get a channel from the pool
		resultCh := resultChPool.Get().(chan T)

		// Send the channel to the taskResults to serialize the results.
		taskResults <- resultCh

		task := func() {
			resultCh <- inTask()
		}

		// If no concurrency limit and the tasks channel is full,
		// create a new worker to handle the task.
		if c == 0 {
			select {
			case tasks <- task:
			default:
				addWorker(task)
			}
		} else {
			// Block until a worker is available.
			tasks <- task
		}

	}

	// Input channel is closed, no more tasks will be sent.
	// Wait for everything to finish
	close(tasks)
	workerWg.Wait()
	close(taskResults)
	serializerWg.Wait()
}
