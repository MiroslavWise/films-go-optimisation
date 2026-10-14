package mime

import (
	"path/filepath"
	"strings"
)

var allowed = map[string]struct{}{
	"image/jpeg":  {},
	"image/jpg":   {},
	"image/pjpeg": {},
	"image/png":   {},
	"image/x-png": {},
	"image/webp":  {},
	"image/avif":  {},
	"image/gif":   {},
}

// DetectImageMIME mirrors next-films detectImageMime: declared type, magic bytes, then extension.
func DetectImageMIME(buf []byte, declaredType, fileName string) string {
	mime := strings.ToLower(strings.TrimSpace(declaredType))
	if _, ok := allowed[mime]; ok {
		switch mime {
		case "image/jpg", "image/pjpeg":
			return "image/jpeg"
		case "image/x-png":
			return "image/png"
		default:
			return mime
		}
	}

	if len(buf) >= 3 && buf[0] == 0xff && buf[1] == 0xd8 && buf[2] == 0xff {
		return "image/jpeg"
	}
	if len(buf) >= 8 && buf[0] == 0x89 && buf[1] == 0x50 && buf[2] == 0x4e && buf[3] == 0x47 {
		return "image/png"
	}
	if len(buf) >= 12 && string(buf[0:4]) == "RIFF" && string(buf[8:12]) == "WEBP" {
		return "image/webp"
	}
	if len(buf) >= 6 {
		head := string(buf[0:6])
		if head == "GIF87a" || head == "GIF89a" {
			return "image/gif"
		}
	}
	if len(buf) >= 12 && string(buf[4:8]) == "ftyp" {
		brand := string(buf[8:12])
		box := string(buf[8:min(32, len(buf))])
		if strings.HasPrefix(brand, "avif") || strings.HasPrefix(brand, "avis") ||
			strings.Contains(box, "avif") || strings.Contains(box, "avis") {
			return "image/avif"
		}
	}

	ext := strings.ToLower(filepath.Ext(fileName))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	case ".avif":
		return "image/avif"
	}
	return ""
}
