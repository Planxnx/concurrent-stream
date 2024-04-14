package cstream

import (
	"context"
	"sync"
	"sync/atomic"
)

// Stream is a simple stream of tasks with a concurrency limit.
type Stream[T any] struct {
	ctx   context.Context
	in    chan func() T
	out   chan T
	done  chan struct{}
	isRun atomic.Bool
}

// NewStream creates a new StreamChan with the given context, concurrency limit, and output channel.
func NewStream[T any](ctx context.Context, c int, out chan T) *Stream[T] {
	in := make(chan func() T)
	done := make(chan struct{})

	s := &Stream[T]{
		ctx:  ctx,
		in:   in,
		out:  out,
		done: done,
	}

	s.isRun.Store(true)
	go func() {
		defer close(done)
		defer s.isRun.Store(false)
		stream(ctx, c, in, out)
	}()

	return s
}

// Close stops the stream. Should be called after all tasks are submitted or want to stop the stream.
// Will block until concurrent map is closed.
func (s *Stream[T]) Close() {
	if !s.isRun.Load() {
		return
	}
	close(s.in)
	<-s.done
}

// IsDone returns true if the stream is done or finished executing.
func (s *Stream[T]) IsDone() bool {
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}

// IsRunning returns true if the stream is running.
func (s *Stream[T]) IsRunning() bool {
	return s.isRun.Load()
}

// Go sends a task to the stream's pool. All tasks are executed concurrently.
// If worker pool is full, it will block until a worker is available.
func (s *Stream[T]) Go(task func() T) {
	if !s.isRun.Load() {
		return
	}
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
	if !s.isRun.Load() {
		return nil
	}
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
	taskResultBufferSize := c
	if c <= 0 {
		c = 0
		taskResultBufferSize = 8
	}

	// Create a pool of result channels
	// to avoid allocating a new channel for each task.
	resultChPool := sync.Pool{
		New: func() any {
			return make(chan T, 1)
		},
	}

	tasks := make(chan func())
	taskResults := make(chan chan T, taskResultBufferSize)

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

		// Create a task that will concurrently execute the input task
		// and return the result to the result channel for serialization.
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
