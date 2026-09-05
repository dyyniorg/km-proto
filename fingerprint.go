package kmproto

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
)

// Docs: https://spec.matrix.org/v1.19/server-server-api/#get_matrixfederationv1version
type versionResponse struct {
	Server struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"server"`
}

// Docs: https://spec.matrix.org/v1.19/server-server-api/#get_matrixkeyv2server
type serverKeysResponse struct {
	ServerName string `json:"server_name"`
	VerifyKeys map[string]struct {
		Key string `json:"key"`
	} `json:"verify_keys"`
}

// Queries a resolved server's version and signing keys, returning a populated Server object. A
// non-nil error is returned only when both endpoints hard-fail. In that case Server object's
// 'Reachable' field will also be flagged as false. Partial responses are returned as-is.
func (c *Client) Fingerprint(ctx context.Context, rs ResolvedServer) (Server, error) {
	base := "https://" + rs.HostPort
	now := time.Now()

	var ver versionResponse
	var keys serverKeysResponse

	verErr := c.GetJSON(ctx, base+"/_matrix/federation/v1/version", rs.HostHeader, rs.CertName, &ver)
	keysErr := c.GetJSON(ctx, base+"/_matrix/key/v2/server", rs.HostHeader, rs.CertName, &keys)

	if verErr != nil && keysErr != nil {
		return Server{
			ServerName:   rs.Name,
			ResolvedHost: rs.HostPort,
			Reachable:    false,
			FirstSeen:    now,
			LastSeen:     now,
		}, fmt.Errorf("fingerprinting %q: version: %w; keys: %w", rs.Name, verErr, keysErr)
	}

	name := rs.Name
	if keys.ServerName != "" {
		name = keys.ServerName
	}

	srv := Server{
		ServerName:   name,
		ResolvedHost: rs.HostPort,
		Reachable:    verErr == nil && keysErr == nil,
		FirstSeen:    now,
		LastSeen:     now,
	}

	if verErr == nil {
		srv.SoftwareName = ver.Server.Name
		srv.SoftwareVersion = ver.Server.Version
	}
	if keysErr == nil {
		srv.KeyFingerprint = fingerprintKeys(keys.VerifyKeys)
	}

	return srv, nil
}

// Produces a sha256 of the canonical (sorted) 'verify_keys' field JSON object as hex.
func fingerprintKeys(verifyKeys map[string]struct {
	Key string `json:"key"`
}) string {
	if len(verifyKeys) == 0 {
		return ""
	}

	ids := make([]string, 0, len(verifyKeys))
	for id := range verifyKeys {
		ids = append(ids, id)
	}
	slices.Sort(ids)

	var b strings.Builder
	b.WriteByte('{')
	for i, id := range ids {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(jsonString(id))
		b.WriteByte(':')
		b.WriteString(jsonString(verifyKeys[id].Key))
	}
	b.WriteByte('}')

	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
