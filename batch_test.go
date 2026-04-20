package go_qr

import (
	"bytes"
	"fmt"
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncodeBatch_PreservesOrder(t *testing.T) {
	inputs := make([]BatchInput, 50)
	for i := range inputs {
		inputs[i] = BatchInput{Text: fmt.Sprintf("item-%d", i), Ecc: Low}
	}
	results := EncodeBatch(inputs, 8)
	assert.Len(t, results, 50)

	// Encode sequentially and compare each code's modules to prove order is stable.
	for i, r := range results {
		assert.NoError(t, r.Err)
		want, err := EncodeText(inputs[i].Text, inputs[i].Ecc)
		assert.NoError(t, err)
		assert.Equal(t, want.GetSize(), r.QR.GetSize())
		for y := 0; y < want.GetSize(); y++ {
			for x := 0; x < want.GetSize(); x++ {
				if want.GetModule(x, y) != r.QR.GetModule(x, y) {
					t.Fatalf("item %d: module mismatch at (%d,%d)", i, x, y)
				}
			}
		}
	}
}

func TestEncodeBatch_PartialFailure(t *testing.T) {
	inputs := []BatchInput{
		{Text: "ok", Ecc: Low},
		{Text: string(make([]byte, 5000)), Ecc: High}, // exceeds v40-High capacity (~1273 bytes)
		{Text: "also ok", Ecc: Low},
	}
	results := EncodeBatch(inputs, 4)
	assert.NoError(t, results[0].Err)
	assert.Error(t, results[1].Err)
	assert.NoError(t, results[2].Err)
	assert.NotNil(t, results[0].QR)
	assert.Nil(t, results[1].QR)
	assert.NotNil(t, results[2].QR)
}

func TestEncodeBatch_EmptyInput(t *testing.T) {
	results := EncodeBatch(nil, 4)
	assert.Len(t, results, 0)
}

func TestEncodeBatch_ConcurrencyDefaultsToCPU(t *testing.T) {
	// Run with concurrency=0 and concurrency=runtime.NumCPU(); both should succeed.
	inputs := []BatchInput{{Text: "a", Ecc: Low}, {Text: "b", Ecc: Low}}
	r1 := EncodeBatch(inputs, 0)
	r2 := EncodeBatch(inputs, runtime.NumCPU())
	assert.NoError(t, r1[0].Err)
	assert.NoError(t, r2[0].Err)
}

func TestRenderBatch_PNG(t *testing.T) {
	jobs := []BatchJob{
		{Text: "one", Ecc: Medium, Format: FormatPNG},
		{Text: "two", Ecc: Medium, Format: FormatPNG},
		{Text: "three", Ecc: Medium, Format: FormatPNG},
	}
	results := RenderBatch(jobs, 4)
	for i, r := range results {
		assert.NoError(t, r.Err, "job %d", i)
		assert.Equal(t, []byte{0x89, 0x50, 0x4e, 0x47}, r.Bytes[:4], "job %d not PNG", i)
	}
}

func TestRenderBatch_SVG(t *testing.T) {
	cfg := NewQrCodeImgConfig(10, 4, WithOptimalSVG())
	jobs := []BatchJob{
		{Text: "one", Ecc: Medium, Format: FormatSVG, Config: cfg},
		{Text: "two", Ecc: Medium, Format: FormatSVG, Config: cfg},
	}
	results := RenderBatch(jobs, 4)
	for i, r := range results {
		assert.NoError(t, r.Err, "job %d", i)
		assert.True(t, bytes.Contains(r.Bytes, []byte("<svg")), "job %d not SVG", i)
		assert.True(t, bytes.Contains(r.Bytes, []byte("fill-rule=\"evenodd\"")), "job %d not optimal SVG", i)
	}
}

func TestRenderBatch_DefaultConfigAndColors(t *testing.T) {
	// Omitting Config and colors should still work.
	jobs := []BatchJob{{Text: "defaults", Ecc: Low, Format: FormatSVG}}
	results := RenderBatch(jobs, 1)
	assert.NoError(t, results[0].Err)
	assert.Contains(t, string(results[0].Bytes), "#FFFFFF")
	assert.Contains(t, string(results[0].Bytes), "#000000")
}

func TestRenderBatch_InvalidFormat(t *testing.T) {
	jobs := []BatchJob{{Text: "x", Ecc: Low, Format: Format(99)}}
	results := RenderBatch(jobs, 1)
	assert.Error(t, results[0].Err)
}

func TestRunWorkers_RunsAllIndices(t *testing.T) {
	var seen [100]int32
	runWorkers(100, 8, func(i int) {
		atomic.AddInt32(&seen[i], 1)
	})
	for i, v := range seen {
		assert.Equal(t, int32(1), v, "index %d not visited exactly once", i)
	}
}

func BenchmarkEncodeBatch_Serial(b *testing.B) {
	inputs := make([]BatchInput, 100)
	for i := range inputs {
		inputs[i] = BatchInput{Text: fmt.Sprintf("hello-%d", i), Ecc: Medium}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		EncodeBatch(inputs, 1)
	}
}

func BenchmarkEncodeBatch_Parallel(b *testing.B) {
	inputs := make([]BatchInput, 100)
	for i := range inputs {
		inputs[i] = BatchInput{Text: fmt.Sprintf("hello-%d", i), Ecc: Medium}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		EncodeBatch(inputs, 0)
	}
}
