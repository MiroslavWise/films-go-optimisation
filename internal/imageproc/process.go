package imageproc

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/buckket/go-blurhash"
	"github.com/disintegration/imaging"
	"github.com/gen2brain/avif"
	_ "golang.org/x/image/webp"
)

type Kind string

const (
	KindPoster   Kind = "poster"
	KindPortrait Kind = "portrait"
	KindFrame    Kind = "frame"
)

const (
	PosterMaxWidth   = 800
	PortraitMaxWidth = 640
	FrameMaxWidth    = 1280

	BlurhashX      = 5
	BlurhashY      = 9
	FrameBlurhashX = 9
	FrameBlurhashY = 5
)

type Options struct {
	Quality int
	Speed   int
}

type Result struct {
	AVIF     []byte
	Blurhash string
	Width    int
	Height   int
}

func ParseKind(s string) (Kind, error) {
	switch Kind(s) {
	case KindPoster, KindPortrait, KindFrame:
		return Kind(s), nil
	default:
		return "", fmt.Errorf("unsupported kind %q", s)
	}
}

func MaxWidth(kind Kind) int {
	switch kind {
	case KindPoster:
		return PosterMaxWidth
	case KindFrame:
		return FrameMaxWidth
	default:
		return PortraitMaxWidth
	}
}

func BlurComponents(kind Kind) (x, y int) {
	if kind == KindFrame {
		return FrameBlurhashX, FrameBlurhashY
	}
	return BlurhashX, BlurhashY
}

// ProcessImage: decode + EXIF orient + resize (no upscale) + flatten alpha + blurhash + AVIF.
func ProcessImage(src []byte, kind Kind, opts Options) (Result, error) {
	if opts.Quality < 1 || opts.Quality > 100 {
		opts.Quality = 38
	}
	if opts.Speed < 0 || opts.Speed > 10 {
		opts.Speed = 6
	}

	img, err := imaging.Decode(bytes.NewReader(src), imaging.AutoOrientation(true))
	if err != nil {
		return Result{}, fmt.Errorf("decode image: %w", err)
	}

	maxW := MaxWidth(kind)
	img = resizeNoUpscale(img, maxW)
	img = flattenAlpha(img)

	bx, by := BlurComponents(kind)
	hash, err := blurhash.Encode(bx, by, img)
	if err != nil {
		return Result{}, fmt.Errorf("blurhash: %w", err)
	}

	var buf bytes.Buffer
	err = avif.Encode(&buf, img, avif.Options{
		Quality:           opts.Quality,
		QualityAlpha:      opts.Quality,
		Speed:             opts.Speed,
		ChromaSubsampling: image.YCbCrSubsampleRatio420,
	})
	if err != nil {
		return Result{}, fmt.Errorf("encode avif: %w", err)
	}

	b := img.Bounds()
	return Result{
		AVIF:     buf.Bytes(),
		Blurhash: hash,
		Width:    b.Dx(),
		Height:   b.Dy(),
	}, nil
}

func resizeNoUpscale(img image.Image, maxWidth int) image.Image {
	b := img.Bounds()
	w := b.Dx()
	if w <= maxWidth || maxWidth <= 0 {
		return img
	}
	h := b.Dy()
	newH := h * maxWidth / w
	if newH < 1 {
		newH = 1
	}
	return imaging.Resize(img, maxWidth, newH, imaging.Lanczos)
}

// flattenAlpha composites onto black (sharp removeAlpha default).
func flattenAlpha(img image.Image) *image.NRGBA {
	b := img.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(out, out.Bounds(), image.Black, image.Point{}, draw.Src)
	draw.Draw(out, out.Bounds(), img, b.Min, draw.Over)
	return out
}

func BlobPath(kind Kind, entityID, movieID string) (string, error) {
	switch kind {
	case KindPoster:
		return "posters/" + entityID + ".avif", nil
	case KindPortrait:
		return "people/" + entityID + ".avif", nil
	case KindFrame:
		if movieID == "" {
			return "", fmt.Errorf("movieId is required for kind=frame")
		}
		return "frame-guess/" + movieID + "/" + entityID + ".avif", nil
	default:
		return "", fmt.Errorf("unsupported kind")
	}
}
