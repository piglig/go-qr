package go_qr

import (
	"runtime"
	"sync"
)

// Format selects the output format for RenderBatch.
type Format int

const (
	// FormatPNG renders to a PNG byte slice.
	FormatPNG Format = iota
	// FormatSVG renders to an SVG byte slice. The WithOptimalSVG option on
	// the job's Config is honored.
	FormatSVG
)

// BatchInput is one QR code to encode.
type BatchInput struct {
	Text string
	Ecc  Ecc
}

// BatchEncodeResult is the result of encoding one BatchInput. Results are
// returned in input order; a failed item's QR is nil and Err describes the
// failure, but other items continue to be processed.
type BatchEncodeResult struct {
	QR  *QrCode
	Err error
}

// EncodeBatch encodes the given inputs concurrently and returns results in
// input order.
//
// concurrency bounds the number of workers; values <= 0 default to
// runtime.NumCPU(). A failure for one input does not cancel the others.
func EncodeBatch(inputs []BatchInput, concurrency int) []BatchEncodeResult {
	results := make([]BatchEncodeResult, len(inputs))
	runWorkers(len(inputs), concurrency, func(i int) {
		qr, err := EncodeText(inputs[i].Text, inputs[i].Ecc)
		results[i] = BatchEncodeResult{QR: qr, Err: err}
	})
	return results
}

// BatchJob is one encode-and-render task for RenderBatch.
type BatchJob struct {
	Text   string
	Ecc    Ecc
	Format Format
	// Config is the rendering configuration. If nil, a default
	// NewQrCodeImgConfig(10, 4) is used. Colors are read from the config
	// (WithLight / WithDark); defaults are white/black.
	Config *QrCodeImgConfig
}

// BatchRenderResult is the result of one BatchJob. QR is the encoded code
// (useful even on render failure for diagnostics); Bytes holds the rendered
// output on success.
type BatchRenderResult struct {
	QR    *QrCode
	Bytes []byte
	Err   error
}

// RenderBatch encodes and renders each job concurrently, returning results
// in input order.
//
// concurrency bounds the number of workers; values <= 0 default to
// runtime.NumCPU(). A failure for one job does not cancel the others.
func RenderBatch(jobs []BatchJob, concurrency int) []BatchRenderResult {
	results := make([]BatchRenderResult, len(jobs))
	runWorkers(len(jobs), concurrency, func(i int) {
		results[i] = renderOne(jobs[i])
	})
	return results
}

func renderOne(job BatchJob) BatchRenderResult {
	qr, err := EncodeText(job.Text, job.Ecc)
	if err != nil {
		return BatchRenderResult{Err: err}
	}
	cfg := job.Config
	if cfg == nil {
		cfg = NewQrCodeImgConfig(10, 4)
	}
	var bytes []byte
	switch job.Format {
	case FormatPNG:
		bytes, err = qr.ToPNGBytes(cfg)
	case FormatSVG:
		bytes, err = qr.ToSVGBytes(cfg)
	default:
		return BatchRenderResult{QR: qr, Err: errInvalidFormat(job.Format)}
	}
	return BatchRenderResult{QR: qr, Bytes: bytes, Err: err}
}

// runWorkers runs fn(i) for i in [0, n) across a bounded worker pool.
// For tiny batches (n == 1 or concurrency == 1) it runs synchronously to
// avoid goroutine overhead.
func runWorkers(n, concurrency int, fn func(int)) {
	if n == 0 {
		return
	}
	if concurrency <= 0 {
		concurrency = runtime.NumCPU()
	}
	if concurrency > n {
		concurrency = n
	}
	if concurrency == 1 {
		for i := 0; i < n; i++ {
			fn(i)
		}
		return
	}

	jobs := make(chan int, concurrency)
	var wg sync.WaitGroup
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				fn(i)
			}
		}()
	}
	for i := 0; i < n; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
}

type invalidFormatError Format

func (e invalidFormatError) Error() string {
	return "go_qr: invalid batch format"
}

func errInvalidFormat(f Format) error { return invalidFormatError(f) }
