package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

type Client struct {
	Secret string
	HTTP   *http.Client
}

func New(secret string) *Client {
	return &Client{
		Secret: secret,
		HTTP: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

// Notify POSTs JSON with Bearer + HMAC (timestamp.body). Retries 3 times: 1s, 3s, 10s.
func (c *Client) Notify(ctx context.Context, webhookURL, jobID string, rawBody []byte) error {
	if webhookURL == "" {
		return fmt.Errorf("empty webhook url")
	}

	delays := []time.Duration{0, time.Second, 3 * time.Second, 10 * time.Second}
	var lastErr error
	for i, delay := range delays {
		if i > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
		lastErr = c.once(ctx, webhookURL, jobID, rawBody)
		if lastErr == nil {
			return nil
		}
	}
	return lastErr
}

func (c *Client) once(ctx context.Context, webhookURL, jobID string, rawBody []byte) error {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(rawBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Job-Id", jobID)
	req.Header.Set("X-Timestamp", ts)
	if c.Secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.Secret)
		mac := hmac.New(sha256.New, []byte(c.Secret))
		_, _ = mac.Write([]byte(ts + "."))
		_, _ = mac.Write(rawBody)
		req.Header.Set("X-Signature", hex.EncodeToString(mac.Sum(nil)))
	}

	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 64<<10))
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("webhook status %d", res.StatusCode)
	}
	return nil
}
