package kmproto

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// defaults recommended by the spec
	// (https://spec.matrix.org/v1.19/server-server-api/#resolving-server-names)
	wellKnownErrTTL = time.Hour
	wellKnownMinTTL = 24 * time.Hour
)

type ResolvedServer struct {
	HostPort   string // "host:port" to dial
	HostHeader string // HTTP Host header value (logical server name, possibly with/without port)
	CertName   string // TLS ServerName (SNI, "" for IP literals)
	Name       string // canonical server name for fingerprinting/signing
}

type resolveEntry struct {
	rs      ResolvedServer
	err     error
	expires time.Time
}

type Resolver struct {
	client *Client
	mu     sync.Mutex
	cache  map[string]resolveEntry
}

func NewResolver(client *Client) *Resolver {
	return &Resolver{client: client, cache: make(map[string]resolveEntry)}
}

// Resolves a Matrix server name to a connect target per the spec's instructions (incl. caching).
//
// Docs:
// - https://spec.matrix.org/v1.19/server-server-api/#resolving-server-names
func (r *Resolver) Resolve(ctx context.Context, name string) (ResolvedServer, error) {
	r.mu.Lock()
	e, ok := r.cache[name]
	r.mu.Unlock()
	if ok && time.Now().Before(e.expires) {
		return e.rs, e.err
	}

	rs, err := r.resolve(ctx, name)

	r.mu.Lock()
	r.cache[name] = resolveEntry{rs: rs, err: err, expires: time.Now().Add(cacheTTL(err))}
	r.mu.Unlock()
	return rs, err
}

// Resolving in the (full) exact order defined by the spec.
func (r *Resolver) resolve(ctx context.Context, name string) (ResolvedServer, error) {
	host, port, hasPort, err := splitServerName(name)
	if err != nil {
		return ResolvedServer{}, err
	}

	// 1. IP literal (direct connection, no SNI/.well-known/SRV)
	if ip := net.ParseIP(host); ip != nil {
		p := port
		if !hasPort {
			p = "8448"
		}
		return ResolvedServer{
			HostPort:   net.JoinHostPort(host, p),
			HostHeader: net.JoinHostPort(host, p), // Host included the port as per spec
			CertName:   "",
			Name:       name,
		}, nil
	}

	// 2. explicit port (resolved via A/AAAA, connection to host:port, SNI = host)
	if hasPort {
		return ResolvedServer{
			HostPort:   net.JoinHostPort(host, port),
			HostHeader: net.JoinHostPort(host, port),
			CertName:   host,
			Name:       name,
		}, nil
	}

	// 3. .well-known
	delegated, ok, wkErr := r.fetchWellKnown(ctx, host)
	if ok {
		return r.resolveDelegated(ctx, name, delegated)
	}
	// 4-6. SRV, fallback to :8448
	rs, rr := r.resolveViaSRV(ctx, name, host)
	if wkErr != nil && rr == nil {
		// surface wkErr if the SRV lookup succeeds
		rr = wkErr
	}
	return rs, rr
}

// Resolved server via API's /.well-known/matrix/server endpoint (m.server field) with SNI=host.
func (r *Resolver) fetchWellKnown(ctx context.Context, host string) (string, bool, error) {
	url := "https://" + host + "/.well-known/matrix/server"
	var v struct {
		MServer string `json:"m.server"`
	}
	if err := r.client.GetJSON(ctx, url, host, host, &v); err != nil {
		return "", false, err
	}
	if v.MServer == "" {
		return "", false, nil
	}
	return v.MServer, true, nil
}

func (r *Resolver) resolveDelegated(ctx context.Context, name, delegated string) (ResolvedServer, error) {
	dhost, dport, dhasPort, err := splitServerName(delegated)
	if err != nil {
		return ResolvedServer{}, err
	}

	// 3.1. delegated IP literal
	if ip := net.ParseIP(dhost); ip != nil {
		p := dport
		if !dhasPort {
			p = "8448"
		}
		return ResolvedServer{
			HostPort:   net.JoinHostPort(dhost, p),
			HostHeader: dhost,
			CertName:   "", // cert is for the IP
			Name:       name,
		}, nil
	}

	// 3.2. delegated host with explicit port
	if dhasPort {
		return ResolvedServer{
			HostPort:   net.JoinHostPort(dhost, dport),
			HostHeader: net.JoinHostPort(dhost, dport),
			CertName:   dhost,
			Name:       name,
		}, nil
	}

	// 3.3./3.5. delegated host without port, SRV on dhost (or default to :8448)
	rs, rr := r.resolveViaSRV(ctx, name, dhost)
	rs.HostHeader = dhost // delegated host (bare), not the SRV target
	rs.CertName = dhost
	rs.Name = name
	return rs, rr
}

func (r *Resolver) resolveViaSRV(ctx context.Context, name, host string) (ResolvedServer, error) {
	if hp, ok := lookupSRV(host, "_matrix-fed._tcp."); ok {
		return ResolvedServer{
			HostPort:   hp,
			HostHeader: host, // Host header is the logical name, not the SRV target
			CertName:   host,
			Name:       name,
		}, nil
	}
	if hp, ok := lookupSRV(host, "_matrix._tcp."); ok { // deprecated
		return ResolvedServer{
			HostPort:   hp,
			HostHeader: host,
			CertName:   host,
			Name:       name,
		}, nil
	}
	return ResolvedServer{
		HostPort:   net.JoinHostPort(host, "8448"),
		HostHeader: host,
		CertName:   host,
		Name:       name,
	}, nil
}

func cacheTTL(err error) time.Duration {
	// NOTE: server's own Cache-Control max-age is ignored for now
	if err != nil {
		return wellKnownErrTTL
	}
	return wellKnownMinTTL
}

func splitServerName(name string) (host, port string, hasPort bool, err error) {
	if strings.HasPrefix(name, "[") {
		// assuming IPv6 literal, could be "[::1]:<port>" or just "[::1]"
		if end := strings.Index(name, "]"); end != -1 {
			host = name[1:end]
			rest := name[end+1:]
			if strings.HasPrefix(rest, ":") {
				return host, rest[1:], true, nil
			}
			return host, "", false, nil
		}
		return "", "", false, fmt.Errorf("invalid server name: %q", name)
	}

	var c int
	if c = strings.Count(name, ":"); c == 0 {
		return name, "", false, nil
	}
	if c == 1 {
		// "<host>:<port>"
		i := strings.LastIndex(name, ":")
		return name[:i], name[i+1:], true, nil
	}
	return "", "", false, fmt.Errorf("invalid server name: %q", name)
}

func lookupSRV(host, service string) (hostPort string, ok bool) {
	_, addrs, err := net.LookupSRV("", "tcp", service+host)
	if err != nil || len(addrs) == 0 {
		return "", false
	}
	a := addrs[0]
	target := strings.TrimSuffix(a.Target, ".")
	return net.JoinHostPort(target, strconv.Itoa(int(a.Port))), true
}
