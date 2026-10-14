package imageproc

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

func TestProcessImagePoster(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 1000, 1500))
	for y := 0; y < 1500; y++ {
		for x := 0; x < 1000; x++ {
			img.Set(x, y, color.NRGBA{R: uint8(x % 255), G: uint8(y % 255), B: 80, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}

	out, err := ProcessImage(buf.Bytes(), KindPoster, Options{Quality: 38, Speed: 8})
	if err != nil {
		t.Fatal(err)
	}
	if out.Width > PosterMaxWidth {
		t.Fatalf("width %d > %d", out.Width, PosterMaxWidth)
	}
	if out.Blurhash == "" {
		t.Fatal("empty blurhash")
	}
	if len(out.AVIF) < 32 {
		t.Fatalf("avif too small: %d", len(out.AVIF))
	}
	// AVIF ftyp brand
	if len(out.AVIF) < 12 || string(out.AVIF[4:8]) != "ftyp" {
		t.Fatalf("not an avif/isom container")
	}
}
