# **concurrent-stream**: High Throughput Generics Stream/Pipeline/Channel Processing in Go.

[![Go Reference](https://pkg.go.dev/badge/github.com/planxnx/concurrent-stream.svg)](https://pkg.go.dev/github.com/planxnx/concurrent-stream)

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

wg := sync.WaitGroup{}
wg.Add(1)
go func() {
	defer wg.Done()
	for result := range results {
		fmt.Println(result)
	}
}()

if err := stream.Wait(); err != nil {
	panic(err)
}
close(results)

wg.Wait()
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
