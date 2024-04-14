package cstream_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	cstream "github.com/planxnx/concurrent-stream"
)

func TestStream(t *testing.T) {
	ctx := context.Background()

	goroutines := []int{-8, -4, -2, -1, 0, 1, 2, 4, 8}
	n := []int{10, 100, 1000}
	basicStream := func(t *testing.T, n int, goroutine int) {
		results := make(chan int)
		stream := cstream.NewStream(ctx, goroutine, results)

		go func() {
			for i := 0; i < n; i++ {
				if !stream.IsRunning() {
					t.Error("expected stream to be running")
				}
				if stream.IsDone() {
					t.Error("expected stream to be not done")
				}
				i := i
				stream.Go(func() int {
					return factorial(i)
				})
			}

			// Close the stream after all the tasks are submitted.
			stream.Close()

			if !stream.IsDone() {
				t.Error("expected stream to be done after close")
			}
			if stream.IsRunning() {
				t.Error("expected stream to be not running after close")
			}
		}()

		i := 0
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			for result := range results {
				if expected := factorial(i); result != expected {
					t.Errorf("expected %d, got %d", expected, result)
				}
				i++
			}
		}()

		if err := stream.Wait(); err != nil {
			t.Error("unexpected error:", err)
		}
		close(results)

		if !stream.IsDone() {
			t.Error("expected stream to be done after Wait() is returned")
		}

		if stream.IsRunning() {
			t.Error("expected stream to be not running")
		}

		wg.Wait()
		if i != n {
			t.Errorf("expected %d results, got %d", n, i)
		}

		// Multiple calls to Close should not panic.
		stream.Close()
		stream.Close()
	}

	for _, goroutine := range goroutines {
		for _, n := range n {
			t.Run(fmt.Sprintf("%dgouroutines,%dn", goroutine, n), func(t *testing.T) {
				basicStream(t, n, goroutine)
			})
		}
	}
}

func factorial(n int) int {
	if n == 0 {
		return 1
	}
	return n * factorial(n-1)
}
