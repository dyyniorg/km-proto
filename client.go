package kmproto

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
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

	cbMu sync.Mutex
	cb   map[string]string // server name -> client-server base URL (no trailing slash)
}

func NewClient(userAgent string) *Client {
	if userAgent == "" {
		userAgent = defaultUserAgent
	}
	return &Client{
		client: &http.Client{
			Timeout:   defaultReqTimeout,
			Transport: http.DefaultTransport.(*http.Transport).Clone(),
		},
		userAgent: userAgent,
		cb:        make(map[string]string),
	}
}

// Returns a http.Client whose TLS ServerName is sni when non-empty.
// Used when the certificate host (sni) differs from the host dialed in rawURL.
func (c *Client) dialClient(sni string) *http.Client {
	if sni == "" {
		return c.client
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, ServerName: sni}
	cl := *c.client
	cl.Transport = tr
	return &cl
}

// Performs a GET request to rawURL and returns the raw response body. The 'host' parameter, when
// non-empty, overrides the request's Host header (needed when server name differs from the
// resolved host:port used to connect, e.g. due to delegation). Custom SNI can be used when the
// cert host ('sni') differs from the dial host ('rawURL').
func (c *Client) Get(ctx context.Context, rawURL, host, sni string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if host != "" {
		req.Host = host
	}

	resp, err := c.dialClient(sni).Do(req)
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
// differs from the resolved host:port used to connect, e.g. due to delegation). Custom SNI can be
// used when the cert host ('sni') differs from the dial host ('rawURL').
func (c *Client) GetJSON(ctx context.Context, rawURL, host, sni string, out any) error {
	body, err := c.Get(ctx, rawURL, host, sni)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decoding json: %w", err)
	}
	return nil
}

// Resolves the client-server base URL for a homeserver name via its
// /.well-known/matrix/client delegation (m.homeserver.base_url), falling back to
// https://<name> when the file is absent. The result is cached; the returned URL
// has no trailing slash. This is separate from the federation target resolved by
// Resolver.Resolve, since the client and federation APIs may live on different
// hosts (e.g. matrix.org vs matrix-federation.matrix.org).
func (c *Client) clientBase(ctx context.Context, name string) (string, error) {
	c.cbMu.Lock()
	if base, ok := c.cb[name]; ok {
		c.cbMu.Unlock()
		return base, nil
	}
	c.cbMu.Unlock()

	base := "https://" + name

	var wk struct {
		Homeserver struct {
			BaseURL string `json:"base_url"`
		} `json:"m.homeserver"`
	}
	if err := c.GetJSON(ctx, base+"/.well-known/matrix/client", "", "", &wk); err == nil && wk.Homeserver.BaseURL != "" {
		base = strings.TrimRight(wk.Homeserver.BaseURL, "/")
	}

	c.cbMu.Lock()
	c.cb[name] = base
	c.cbMu.Unlock()
	return base, nil
}
