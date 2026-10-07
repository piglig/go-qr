package qr

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
)

func TestBatchPreservesOrder(t *testing.T) {
	jobs := make([]BatchJob, 50)
	for i := range jobs {
		jobs[i] = BatchJob{Text: fmt.Sprintf("item-%d", i), Encode: []EncodeOption{WithECC(ECCLow)}}
	}
	results := Batch(context.Background(), jobs, 8)
	assertLen(t, results, len(jobs))
	for i, r := range results {
		assertNoError(t, r.Err)
		want, err := Encode(jobs[i].Text, jobs[i].Encode...)
		assertNoError(t, err)
		assertEqual(t, want, r.Code, "job %d", i)
		assertNil(t, r.Data, "FormatNone must not render")
	}
}

func TestBatchPartialFailure(t *testing.T) {
	results := Batch(context.Background(), []BatchJob{
		{Text: "ok"},
		{Text: strings.Repeat("x", 5000), Encode: []EncodeOption{WithECC(ECCHigh)}},
		{Text: "also ok", Format: FormatPNG},
	}, 4)
	assertNoError(t, results[0].Err)
	assertTrue(t, errors.Is(results[1].Err, ErrDataTooLong))
	assertNil(t, results[1].Code)
	assertNoError(t, results[2].Err)
	assertNotEmpty(t, results[2].Data)
}

func TestBatchFormats(t *testing.T) {
	results := Batch(context.Background(), []BatchJob{
		{Text: "png", Format: FormatPNG, Render: []RenderOption{WithScale(2)}},
		{Text: "svg", Format: FormatSVG},
	}, 0)
	assertNoError(t, results[0].Err)
	assertTrue(t, bytes.HasPrefix(results[0].Data, []byte("\x89PNG")))
	png, _ := results[0].Code.PNG(WithScale(2))
	assertEqual(t, png, results[0].Data)

	assertNoError(t, results[1].Err)
	assertTrue(t, bytes.HasPrefix(results[1].Data, []byte("<svg")))
}

func TestBatchRenderFailureKeepsCode(t *testing.T) {
	results := Batch(context.Background(), []BatchJob{
		{Text: "x", Format: FormatPNG, Render: []RenderOption{WithScale(0)}},
		{Text: "x", Format: Format(99)},
	}, 1)
	for i, r := range results {
		assertTrue(t, errors.Is(r.Err, ErrInvalidArgument), "job %d: %v", i, r.Err)
		assertNotNil(t, r.Code, "job %d", i)
	}
}

func TestBatchEmpty(t *testing.T) {
	assertLen(t, Batch(context.Background(), nil, 4), 0)
}

func TestBatchCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	results := Batch(ctx, []BatchJob{{Text: "a"}, {Text: "b"}}, 2)
	for _, r := range results {
		assertTrue(t, errors.Is(r.Err, context.Canceled))
		assertNil(t, r.Code)
	}
}

func TestBatchRecoversPanics(t *testing.T) {
	panicky := func(*encodeConfig) { panic("boom") }
	results := Batch(context.Background(), []BatchJob{{Text: "a", Encode: []EncodeOption{panicky}}, {Text: "b"}}, 2)
	assertContains(t, fmt.Sprint(results[0].Err), "panicked: boom")
	assertNoError(t, results[1].Err)
}

func TestRunWorkersRunsAllIndices(t *testing.T) {
	for _, workers := range []int{-1, 0, 1, 3, 100} {
		var sum atomic.Int64
		runWorkers(context.Background(), 10, workers, func(i int) { sum.Add(int64(i)) }, func(int, error) {
			t.Error("skip called without cancellation")
		})
		assertEqual(t, int64(45), sum.Load(), "workers=%d", workers)
	}
}

func benchmarkBatch(b *testing.B, concurrency int) {
	jobs := make([]BatchJob, 100)
	for i := range jobs {
		jobs[i] = BatchJob{Text: fmt.Sprintf("hello-%d", i)}
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Batch(context.Background(), jobs, concurrency)
	}
}

func BenchmarkBatchSerial(b *testing.B)   { benchmarkBatch(b, 1) }
func BenchmarkBatchParallel(b *testing.B) { benchmarkBatch(b, 0) }
