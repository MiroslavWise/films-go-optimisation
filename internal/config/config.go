package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	DefaultMaxUploadBytes = 4 * 1024 * 1024
	DefaultAVIFQuality    = 38
	DefaultAVIFSpeed      = 6
	DefaultPort           = "8080"
)

type Config struct {
	Port                string
	ProcessorToken      string
	JobJWTSecret        string
	BlobToken           string
	WebhookSecret       string
	DefaultWebhookURL   string
	AVIFQuality         int
	AVIFSpeed           int
	MaxUploadBytes      int64
	JobJWTIssuer        string
	JobJWTAudience      string
}

func Load() (Config, error) {
	cfg := Config{
		Port:              envOr("PORT", DefaultPort),
		ProcessorToken:    strings.TrimSpace(os.Getenv("PHOTO_PROCESSOR_TOKEN")),
		JobJWTSecret:      strings.TrimSpace(os.Getenv("PHOTO_JOB_JWT_SECRET")),
		BlobToken:         strings.TrimSpace(os.Getenv("BLOB_READ_WRITE_TOKEN")),
		WebhookSecret:     strings.TrimSpace(os.Getenv("PHOTO_WEBHOOK_SECRET")),
		DefaultWebhookURL: strings.TrimSpace(os.Getenv("DEFAULT_WEBHOOK_URL")),
		AVIFQuality:       envInt("AVIF_QUALITY", DefaultAVIFQuality),
		AVIFSpeed:         envInt("AVIF_SPEED", DefaultAVIFSpeed),
		MaxUploadBytes:    int64(envInt("MAX_UPLOAD_BYTES", DefaultMaxUploadBytes)),
		JobJWTIssuer:      strings.TrimSpace(os.Getenv("JOB_JWT_ISSUER")),
		JobJWTAudience:    strings.TrimSpace(os.Getenv("JOB_JWT_AUDIENCE")),
	}

	if cfg.ProcessorToken == "" && cfg.JobJWTSecret == "" {
		return cfg, fmt.Errorf("set PHOTO_PROCESSOR_TOKEN and/or PHOTO_JOB_JWT_SECRET")
	}
	if cfg.BlobToken == "" {
		return cfg, fmt.Errorf("BLOB_READ_WRITE_TOKEN is required")
	}
	if cfg.AVIFQuality < 1 || cfg.AVIFQuality > 100 {
		cfg.AVIFQuality = DefaultAVIFQuality
	}
	if cfg.AVIFSpeed < 0 || cfg.AVIFSpeed > 10 {
		cfg.AVIFSpeed = DefaultAVIFSpeed
	}
	if cfg.MaxUploadBytes <= 0 {
		cfg.MaxUploadBytes = DefaultMaxUploadBytes
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return n
}
