package qr

import (
	"context"
	"fmt"
	"runtime"
	"sync"
)

// Format selects the output of a BatchJob.
type Format int

const (
	FormatNone Format = iota // encode only; BatchResult.Data is nil
	FormatPNG                // PNG bytes, as returned by Code.PNG
	FormatSVG                // SVG bytes, as returned by Code.SVG
)

// BatchJob is one text to encode, and optionally render, in Batch.
type BatchJob struct {
	Text   string
	Encode []EncodeOption
	Render []RenderOption // used when Format is not FormatNone
	Format Format
}

// BatchResult is the outcome of one BatchJob. On failure Err is set; Code is
// still set when encoding succeeded but rendering failed.
type BatchResult struct {
	Code *Code
	Data []byte
	Err  error
}

// Batch encodes and renders jobs concurrently and returns the results in job
// order. concurrency bounds the number of workers; values <= 0 mean
// runtime.GOMAXPROCS(0). A failing job does not stop the others. When ctx is
// canceled, jobs that have not started fail with ctx.Err().
func Batch(ctx context.Context, jobs []BatchJob, concurrency int) []BatchResult {
	results := make([]BatchResult, len(jobs))
	runWorkers(ctx, len(jobs), concurrency, func(i int) {
		results[i] = runJob(jobs[i])
	}, func(i int, err error) {
		results[i] = BatchResult{Err: err}
	})
	return results
}

func runJob(job BatchJob) (res BatchResult) {
	defer func() {
		if r := recover(); r != nil {
			res = BatchResult{Err: fmt.Errorf("qr: batch job panicked: %v", r)}
		}
	}()

	code, err := Encode(job.Text, job.Encode...)
	if err != nil {
		return BatchResult{Err: err}
	}
	res.Code = code
	switch job.Format {
	case FormatNone:
	case FormatPNG:
		res.Data, res.Err = code.PNG(job.Render...)
	case FormatSVG:
		res.Data, res.Err = code.SVG(job.Render...)
	default:
		res.Err = fmt.Errorf("%w: unknown batch format %d", ErrInvalidArgument, job.Format)
	}
	return res
}

// runWorkers calls do(i) for every i in [0, n) on up to concurrency
// goroutines. Indices not started before ctx is done get skip(i, ctx.Err()).
func runWorkers(ctx context.Context, n, concurrency int, do func(int), skip func(int, error)) {
	if concurrency <= 0 {
		concurrency = runtime.GOMAXPROCS(0)
	}
	concurrency = min(concurrency, n)

	next := make(chan int)
	var wg sync.WaitGroup
	wg.Add(concurrency)
	for range concurrency {
		go func() {
			defer wg.Done()
			for i := range next {
				if err := ctx.Err(); err != nil {
					skip(i, err)
					continue
				}
				do(i)
			}
		}()
	}
	for i := range n {
		next <- i
	}
	close(next)
	wg.Wait()
}
