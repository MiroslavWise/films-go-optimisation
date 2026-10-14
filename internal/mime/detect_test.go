package mime

import "testing"

func TestDetectJPEGMagic(t *testing.T) {
	buf := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10}
	got := DetectImageMIME(buf, "application/octet-stream", "x.bin")
	if got != "image/jpeg" {
		t.Fatalf("got %q", got)
	}
}

func TestDetectByExt(t *testing.T) {
	got := DetectImageMIME(nil, "", "shot.PNG")
	if got != "image/png" {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizeDeclared(t *testing.T) {
	got := DetectImageMIME(nil, "image/jpg", "a.jpg")
	if got != "image/jpeg" {
		t.Fatalf("got %q", got)
	}
}
