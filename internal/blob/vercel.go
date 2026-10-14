package blob

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const apiURL = "https://vercel.com/api/blob"

type Client struct {
	Token  string
	HTTP   *http.Client
}

type PutResult struct {
	URL              string `json:"url"`
	Pathname         string `json:"pathname"`
	ContentType      string `json:"contentType"`
	ContentDisposition string `json:"contentDisposition"`
}

func New(token string) *Client {
	return &Client{
		Token: token,
		HTTP: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Put uploads bytes to a private Vercel Blob pathname (overwrite, no random suffix).
func (c *Client) Put(ctx context.Context, pathname string, data []byte, contentType string) (PutResult, error) {
	pathname = strings.TrimLeft(strings.TrimSpace(pathname), "/")
	if pathname == "" {
		return PutResult{}, fmt.Errorf("empty pathname")
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	u, err := url.Parse(apiURL)
	if err != nil {
		return PutResult{}, err
	}
	q := u.Query()
	q.Set("pathname", pathname)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u.String(), bytes.NewReader(data))
	if err != nil {
		return PutResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	// Match @vercel/blob SDK (≥ private stores): version 12 + x-vercel-blob-access.
	req.Header.Set("x-api-version", "12")
	req.Header.Set("x-content-type", contentType)
	req.Header.Set("x-vercel-blob-access", "private")
	req.Header.Set("x-add-random-suffix", "0")
	req.Header.Set("x-allow-overwrite", "1")
	req.ContentLength = int64(len(data))

	res, err := c.HTTP.Do(req)
	if err != nil {
		return PutResult{}, fmt.Errorf("blob put: %w", err)
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return PutResult{}, fmt.Errorf("blob put status %d: %s", res.StatusCode, truncate(string(body), 300))
	}

	var out PutResult
	if err := json.Unmarshal(body, &out); err != nil {
		return PutResult{}, fmt.Errorf("blob put decode: %w", err)
	}
	if out.URL == "" {
		return PutResult{}, fmt.Errorf("blob put: empty url in response")
	}
	return out, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
