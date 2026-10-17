package qr

import (
	"context"
	"errors"
	"image"
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

// countdownContext is a context that is done after its Err has been asked
// n times: it cancels a decode at each of its checks in turn.
type countdownContext struct {
	context.Context
	n atomic.Int64
}

func newCountdown(n int) *countdownContext {
	c := &countdownContext{Context: context.Background()}
	c.n.Store(int64(n))
	return c
}

func (c *countdownContext) Err() error {
	if c.n.Add(-1) < 0 {
		return context.Canceled
	}
	return nil
}

func TestDecodeContextCanceled(t *testing.T) {
	img := mustImage(t, mustEncode(t, "canceled"), WithScale(4))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if res, err := DecodeContext(ctx, img); res != nil || !errors.Is(err, context.Canceled) {
		t.Errorf("DecodeContext = %v, %v; want context.Canceled", res, err)
	}
	if rs, err := DecodeAllContext(ctx, img); rs != nil || !errors.Is(err, context.Canceled) {
		t.Errorf("DecodeAllContext = %v, %v; want context.Canceled", rs, err)
	}
}

// TestDecodeContextStopsSearching cancels a search at each of its checks:
// every decode either reads the symbol or stops with the context's error,
// and one canceled late enough reads it.
func TestDecodeContextStopsSearching(t *testing.T) {
	code := mustEncode(t, "stop searching when told")
	sym := rotateGray(mustImage(t, code, WithScale(3)), 0.3)
	img := image.NewGray(image.Rect(0, 0, 1600, 1200))
	for i := range img.Pix {
		img.Pix[i] = 200
	}
	sb := sym.Bounds()
	for y := 0; y < sb.Dy(); y++ {
		copy(img.Pix[(400+y)*img.Stride+700:], sym.Pix[y*sym.Stride:y*sym.Stride+sb.Dx()])
	}
	read := false
	for n := 0; n < 64 && !read; n++ {
		res, err := DecodeContext(newCountdown(n), img)
		switch {
		case err == nil:
			assertEqual(t, "stop searching when told", res.Text, "text")
			read = true
		case !errors.Is(err, context.Canceled):
			t.Fatalf("canceled at check %d: %v, want context.Canceled", n, err)
		}
	}
	if !read {
		t.Fatal("never read the symbol")
	}
}

// TestDecodeAllContextPartial expects DecodeAllContext to return the
// symbols it read before it was canceled.
func TestDecodeAllContextPartial(t *testing.T) {
	want := numbered(6, "part %d")
	img, _ := sheet(t, want, 3, 200, 5)
	// The checks a search that runs to the end makes.
	const many = 1 << 30
	ctx := newCountdown(many)
	rs, err := DecodeAllContext(ctx, img)
	assertNoError(t, err)
	assertEqual(t, want, texts(rs), "all symbols")
	checks := int(many - ctx.n.Load())

	partial := false
	for n := 0; n < checks; n += max(1, checks/50) {
		rs, err := DecodeAllContext(newCountdown(n), img)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled at check %d of %d: %v, want context.Canceled", n, checks, err)
		}
		for _, r := range rs {
			if !slices.Contains(want, r.Text) {
				t.Fatalf("canceled at check %d: read %q", n, r.Text)
			}
		}
		partial = partial || len(rs) > 0 && len(rs) < len(want)
	}
	if !partial {
		t.Error("no cancellation returned some of the symbols")
	}
}

// TestDecodeContextNoSymbol cancels the search of an image without a
// symbol, which runs every stage, at each of its checks: each returns the
// context's error.
func TestDecodeContextNoSymbol(t *testing.T) {
	img := benchPhoto(400, 300, false)
	const many = 1 << 30
	ctx := newCountdown(many)
	if _, err := DecodeContext(ctx, img); errors.Is(err, context.Canceled) || err == nil {
		t.Fatalf("uncanceled search: %v", err)
	}
	checks := int(many - ctx.n.Load())
	for n := 0; n < checks; n++ {
		if _, err := DecodeContext(newCountdown(n), img); !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled at check %d of %d: %v, want context.Canceled", n, checks, err)
		}
	}
}

// TestDecodeContextDeadline bounds the time spent on a large image without
// a symbol, the slowest case of the search.
func TestDecodeContextDeadline(t *testing.T) {
	if testing.Short() || raceEnabled {
		t.Skip("times a large image")
	}
	img := benchPhoto(4000, 3000, false)
	start := time.Now()
	full, err := Decode(img)
	if full != nil || err == nil {
		t.Fatal("decoded an image without a symbol")
	}
	whole := time.Since(start)

	ctx, cancel := context.WithTimeout(context.Background(), whole/10)
	defer cancel()
	start = time.Now()
	_, err = DecodeContext(ctx, img)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
	if took := time.Since(start); took > whole/2 {
		t.Errorf("canceled after %v of a %v search, want well before its end", took, whole)
	}
}
