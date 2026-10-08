package bench

import (
	"encoding/json"
	"flag"
	"image"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/makiuchi-d/gozxing"
	multiqr "github.com/makiuchi-d/gozxing/multi/qrcode"
	"github.com/piglig/go-qr/v2"
)

var (
	boofcvDir = flag.String("boofcv", "", "directory of the BoofCV QR detection dataset (qrcodes/detection)")
	boofcvOut = flag.String("boofcv-out", "boofcv-go.jsonl", "where TestBoofCV writes its results")
)

// TestBoofCV decodes every image of the BoofCV QR Code detection dataset
// (https://boofcv.org/index.php?title=Performance:QrCode) with go-qr and
// gozxing, and writes one JSON line per decoder and image: the category,
// the image, the decoded texts and the decode time. The dataset is not part
// of the repository; download qrcodes_v3.zip and pass its detection
// directory:
//
//	go test -run=TestBoofCV -timeout=60m ./bench/ -boofcv=/path/to/qrcodes/detection
func TestBoofCV(t *testing.T) {
	if *boofcvDir == "" {
		t.Skip("pass -boofcv to run")
	}
	out, err := os.Create(*boofcvOut)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)

	cats, err := os.ReadDir(*boofcvDir)
	if err != nil {
		t.Fatal(err)
	}
	type row struct {
		Dec   string   `json:"dec"`
		Cat   string   `json:"cat"`
		Img   string   `json:"img"`
		Texts []string `json:"texts"`
		Ms    float64  `json:"ms"`
	}
	decoders := []struct {
		name   string
		decode func(image.Image) []string
	}{
		{"go-qr", func(img image.Image) []string {
			if res, err := qr.Decode(img); err == nil {
				return []string{res.Text}
			}
			return nil
		}},
		{"gozxing", decodeGozxingMulti},
	}
	for _, c := range cats {
		if !c.IsDir() {
			continue
		}
		files, _ := filepath.Glob(filepath.Join(*boofcvDir, c.Name(), "*"))
		sort.Strings(files)
		for _, f := range files {
			ext := strings.ToLower(filepath.Ext(f))
			if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
				continue
			}
			img, err := loadGray(f)
			if err != nil {
				t.Errorf("%s: %v", f, err)
				continue
			}
			for _, d := range decoders {
				start := time.Now()
				texts := d.decode(img)
				ms := float64(time.Since(start).Microseconds()) / 1000
				if texts == nil {
					texts = []string{}
				}
				if err := enc.Encode(row{d.name, c.Name(), filepath.Base(f), texts, ms}); err != nil {
					t.Fatal(err)
				}
			}
		}
		t.Logf("%s done", c.Name())
	}
}

// loadGray decodes an image file to grayscale, as the Python decoders read
// it, so every decoder times decoding alone.
func loadGray(path string) (*image.Gray, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	src, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	g := image.NewGray(image.Rect(0, 0, src.Bounds().Dx(), src.Bounds().Dy()))
	draw.Draw(g, g.Bounds(), src, src.Bounds().Min, draw.Src)
	return g, nil
}

func decodeGozxingMulti(img image.Image) []string {
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return nil
	}
	hints := map[gozxing.DecodeHintType]interface{}{gozxing.DecodeHintType_TRY_HARDER: true}
	res, err := multiqr.NewQRCodeMultiReader().DecodeMultiple(bmp, hints)
	if err != nil {
		return nil
	}
	texts := make([]string, len(res))
	for i, r := range res {
		texts[i] = r.GetText()
	}
	return texts
}
