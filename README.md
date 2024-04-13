# **concurrent-stream**: High Throughput Generics Stream/Pipeline/Channel Processing in Go.

```sh
go get github.com/planxnx/concurrent-stream
```

## Examples

### Basic

```go
	results := make(chan int)
	stream := cstream.NewStream(ctx, 8, results)

	for i := 0; i < 10; i++ {
		i := i
		stream.Go(func() int {
			return expensiveFunc(i)
		})
	}

	go func() {
		for result := range results {
			fmt.Pritln(result)
        }
	}

	if err := stream.Wait(); err != nil {
		panic(err)
	}
	close(result)
```

### Concurrency Mapping

```go
	data := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	mapper := NewParallelMap(ctx, 8, data, func(item int, _ int) {
		return expensiveFunc(item)
	})

	.
	.
	.

	result, err := mapper.Result()
	if err != nil {
		panic(err)
	}
```
