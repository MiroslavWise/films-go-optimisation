package imageproc

import "testing"

func TestBlobPath(t *testing.T) {
	p, err := BlobPath(KindPoster, "0193aaaaaaaaaaaaaaaaaaaaaaaa", "")
	if err != nil || p != "posters/0193aaaaaaaaaaaaaaaaaaaaaaaa.avif" {
		t.Fatalf("poster: %q %v", p, err)
	}
	p, err = BlobPath(KindFrame, "f1", "m1")
	if err != nil || p != "frame-guess/m1/f1.avif" {
		t.Fatalf("frame: %q %v", p, err)
	}
	_, err = BlobPath(KindFrame, "f1", "")
	if err == nil {
		t.Fatal("expected error for frame without movieId")
	}
}
