package cstream_test

import (
	"context"
	"fmt"
	"testing"

	cstream "github.com/planxnx/concurrent-stream"
)

func TestMap(t *testing.T) {
	ctx := context.Background()

	goroutines := []int{-8, -4, -2, -1, 0, 1, 2, 4, 8}
	n := []int{1, 10, 100, 1000, 10000}
	basicStream := func(t *testing.T, n []int, goroutine int) {
		mapper := cstream.NewMap(ctx, goroutine, n, func(item int, _ int) int {
			return factorial(item)
		})

		result, err := mapper.Result(ctx)
		if err != nil {
			t.Error("unexpected error:", err)
			t.FailNow()
		}

		if mapper.IsRunning() {
			t.Error("expected not running")
		}

		if !mapper.IsDone() {
			t.Error("expected done")
		}

		if len(result) != len(n) {
			t.Errorf("expected %d items, got %d", len(n), len(result))
		}

		for i, item := range result {
			if expected := factorial(i); item != expected {
				t.Errorf("expected %d, got %d", expected, item)
			}
		}

		// Multiple calls to Close should not panic.
		mapper.Close()
		mapper.Close()
	}

	inputs := make([][]int, 0, len(n))
	for _, n := range n {
		input := make([]int, 0, n)
		for i := 0; i < n; i++ {
			input = append(input, i)
		}
		inputs = append(inputs, input)
	}

	for _, goroutine := range goroutines {
		for _, input := range inputs {
			t.Run(fmt.Sprintf("%dgouroutines,%dn", goroutine, len(input)), func(t *testing.T) {
				basicStream(t, input, goroutine)
			})
		}
	}
}
