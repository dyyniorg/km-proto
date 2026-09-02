package kmproto

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultUserAgent  = "kmproto/0.1 a@a.cc"
	defaultReqTimeout = 30 * time.Second
)

type HTTPError struct {
	StatusCode int
	Status     string
	Body       []byte
}

func (he *HTTPError) Error() string {
	return fmt.Sprintf("http %d %s: %s", he.StatusCode, he.Status, string(he.Body))
}

type Client struct {
	client    *http.Client
	userAgent string
}

func NewClient(userAgent string) *Client {
	if userAgent == "" {
		userAgent = defaultUserAgent
	}
	return &Client{
		client: &http.Client{
			Timeout: defaultReqTimeout,
			// NOTE: server name indication (SNI) doesn't match when the cert host differs from the actual
			// 		 connection host due to delegation -> needs to be overriden in thos ecases (should be
			// 		 handled in discovery.go)
			Transport: &http.Transport{},
		},
		userAgent: userAgent,
	}
}

// Performs a GET request to rawURL and returns the raw response body. The 'host' parameter, when
// non-empty, overrides the request's Host header (needed when server name differs from the
// resolved host:port used to connect, e.g. due to delegation).
func (c *Client) Get(ctx context.Context, rawURL, host string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if host != "" {
		req.Host = host
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 2 MB hard ceiling
	if err != nil {
		return nil, fmt.Errorf("response body read: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &HTTPError{StatusCode: resp.StatusCode, Status: resp.Status, Body: body}
	}
	return body, nil
}

// Performs a GET request to rawURL (using c.Get()) and decodes the JSON response into out. The
// 'host' parameter, when non-empty, overrides the request's Host header (needed when server name
// differs from the resolved host:port used to connect, e.g. due to delegation).
func (c *Client) GetJSON(ctx context.Context, rawURL, host string, out any) error {
	body, err := c.Get(ctx, rawURL, host)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decoding json: %w", err)
	}
	return nil
}
