package cstream

import (
	"context"
	"sync/atomic"
)

// Map is concurrent map function with a concurrency limit.
type Map[I, O any] struct {
	ctx       context.Context
	goroutine int // TODO: support dynamic goroutine limit (create Limiter instance)
	input     []I
	iteratee  func(item I, index int) O
	in        chan func() O
	out       chan O
	result    []O
	done      chan struct{}
	quit      chan struct{}
	isRun     atomic.Bool
}

// NewMap creates a new concurrent map with the given context, concurrency limit, input slice, and callback function.
func NewMap[I, O any](ctx context.Context, goroutine int, input []I, iteratee func(item I, index int) O) *Map[I, O] {
	in := make(chan func() O)
	out := make(chan O, len(input))
	done := make(chan struct{})
	quit := make(chan struct{})

	m := &Map[I, O]{
		ctx:       ctx,
		input:     input,
		iteratee:  iteratee,
		in:        in,
		out:       out,
		result:    make([]O, 0, len(input)),
		done:      done,
		quit:      quit,
		goroutine: goroutine,
	}
	m.run()
	return m
}

func (m *Map[I, O]) run() {
	if m.isRun.Load() {
		return
	}
	m.isRun.Store(true)

	// Start the input stream.
	// After all the input is processed or got canceled, close the input channel to stop the stream.
	go func() {
		defer close(m.in)
		for i, item := range m.input {
			i, item := i, item
			select {
			case <-m.done:
				return
			case <-m.quit:
				return
			case m.in <- func() O {
				return m.iteratee(item, i)
			}:
			}
		}
	}()

	// Start the stream.
	// Stream will stop when the input channel is closed.
	go func() {
		defer m.isRun.Store(false)
		defer close(m.done)
		defer close(m.out)
		stream(m.ctx, m.goroutine, m.in, m.out)
	}()
}

// Close stops the concurrent map.
// Will block until concurrent map is closed.
func (m *Map[I, O]) Close() {
	if !m.isRun.Load() {
		return
	}
	select {
	case <-m.done:
	case m.quit <- struct{}{}:
		<-m.done
	}
}

// IsDone returns true if the concurrent map is done or closed.
func (m *Map[I, O]) IsDone() bool {
	select {
	case <-m.done:
		return true
	default:
		return false
	}
}

// IsRunning returns true if the stream is running.
func (m *Map[I, O]) IsRunning() bool {
	return m.isRun.Load()
}

// Wait blocks until map is done or the context is canceled.
func (m *Map[I, O]) Wait() error {
	if !m.isRun.Load() {
		return nil
	}
	select {
	case <-m.done:
		return nil
	case <-m.ctx.Done():
		return m.ctx.Err()
	}
}

// Result returns the results of the parallel map.
// It will block until all the tasks are completed.
func (m *Map[I, O]) Result(ctx context.Context) ([]O, error) {
	for {
		select {
		case o, ok := <-m.out:
			if !ok {
				return m.result, nil
			}
			m.result = append(m.result, o)
		case <-ctx.Done():
			return m.result, ctx.Err()
		}
	}
}
