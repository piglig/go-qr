# Batch processing

## Concurrency

Every function and method in the package is safe for concurrent use, and a
`*qr.Code` is immutable once returned, so you can encode and render from as
many goroutines as you like. The encoder shares per-version tables across
calls without locking.

## Batch

`Batch` runs a slice of jobs on a worker pool and returns the results in job
order. Each job has its own text, encode options, render options and output
format:

```go
results := qr.Batch(ctx, []qr.BatchJob{
	{Text: "https://example.com/1", Format: qr.FormatPNG},
	{Text: "https://example.com/2", Format: qr.FormatSVG,
		Encode: []qr.EncodeOption{qr.WithECC(qr.ECCHigh)},
		Render: []qr.RenderOption{qr.WithScale(4)}},
	{Text: "encode only"}, // FormatNone: Code is set, Data is nil
}, 0)

for i, r := range results {
	if r.Err != nil {
		log.Printf("job %d: %v", i, r.Err)
		continue
	}
	save(i, r.Data)
}
```

- The last argument is the number of workers; 0 or less means
  `runtime.GOMAXPROCS(0)`.
- A failing job does not stop the others. When encoding succeeded but
  rendering failed, `Code` is still set.
- Canceling `ctx` skips jobs that have not started; they fail with
  `ctx.Err()`.
- A panic in a job is reported as that job's error.

## Throughput tips

- **Prefer SVG for styled codes.** It costs a fraction of an RGBA PNG.
- **Reuse option slices.** Build `[]qr.RenderOption` once and pass it to
  every job.
- **Keep the scale modest** for PNG. Time grows with the number of pixels;
  `WithScale(4)` is plenty for screens.
