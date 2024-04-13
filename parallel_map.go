package cstream

import "context"

// ParallelMap is parallel map function with a concurrency limit.
type ParallelMap[I, O any] struct {
	ctx    context.Context
	in     chan func() O
	out    chan O
	result []O
	done   chan struct{}
}

// NewParallelMap creates a new ParallelMap with the given context, concurrency limit, input slice, and callback function.
func NewParallelMap[I, O any](ctx context.Context, c int, input []I, iteratee func(item I, index int) O) ParallelMap[I, O] {
	in := make(chan func() O)
	out := make(chan O, len(input))
	done := make(chan struct{})
	go func() {
		defer close(in)
		for i, item := range input {
			i, item := i, item
			in <- func() O {
				return iteratee(item, i)
			}
		}
	}()
	go func() {
		defer close(done)
		stream(ctx, c, in, out)
	}()

	return ParallelMap[I, O]{
		in:     in,
		out:    out,
		result: make([]O, 0, len(input)),
		done:   done,
	}
}

// Close stops the stream.
func (s *ParallelMap[I, O]) Close() {
	close(s.in)
	close(s.done)
}

// Wait blocks until map is done or the context is canceled.
func (p *ParallelMap[I, O]) Wait() error {
	select {
	case <-p.done:
		return nil
	case <-p.ctx.Done():
		return p.ctx.Err()
	}
}

// Result returns the results of the parallel map.
// It will block until all the tasks are completed.
func (p *ParallelMap[I, O]) Result(ctx context.Context) ([]O, error) {
	defer close(p.out)
	for {
		select {
		case <-p.done:
			return p.result, nil
		case o := <-p.out:
			p.result = append(p.result, o)
		case <-ctx.Done():
			return p.result, ctx.Err()
		}
	}
}
