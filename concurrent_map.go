package cstream

import "context"

// Map is concurrent map function with a concurrency limit.
type Map[I, O any] struct {
	ctx    context.Context
	in     chan func() O
	out    chan O
	result []O
	done   chan struct{}
}

// NewMap creates a new concurrent map with the given context, concurrency limit, input slice, and callback function.
func NewMap[I, O any](ctx context.Context, goroutine int, input []I, iteratee func(item I, index int) O) Map[I, O] {
	in := make(chan func() O)
	out := make(chan O, len(input))
	done := make(chan struct{})
	go func() {
		defer close(in)
		for i, item := range input {
			i, item := i, item
			select {
			case <-done:
				return
			case in <- func() O {
				return iteratee(item, i)
			}:
			}
		}
	}()
	go func() {
		defer close(done)
		stream(ctx, goroutine, in, out)
	}()

	return Map[I, O]{
		in:     in,
		out:    out,
		result: make([]O, 0, len(input)),
		done:   done,
	}
}

// Close stops the concurrent map.
func (m *Map[I, O]) Close() {
	close(m.in)
	close(m.done)
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

// Wait blocks until map is done or the context is canceled.
func (m *Map[I, O]) Wait() error {
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
	defer close(m.out)
	for {
		select {
		case <-m.done:
			return m.result, nil
		case o := <-m.out:
			m.result = append(m.result, o)
		case <-ctx.Done():
			return m.result, ctx.Err()
		}
	}
}
