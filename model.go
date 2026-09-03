package kmproto

import (
	"strings"
	"time"
)

// Homeserver discovered via the federation (server-to-server) interfaces.
//
// Docs:
// - https://spec.matrix.org/v1.19/server-server-api/#getwell-knownmatrixserver (resolving)
// - https://spec.matrix.org/v1.19/server-server-api/#resolving-server-names (resolving)
// - https://spec.matrix.org/v1.19/server-server-api/#get_matrixfederationv1version (software)
// - https://spec.matrix.org/v1.19/server-server-api/#get_matrixkeyv2server (fingerprinting)
type Server struct {
	ServerName      string    `json:"server_name"`
	ResolvedHost    string    `json:"resolved_host,omitempty"`
	SoftwareName    string    `json:"software_name,omitempty"`
	SoftwareVersion string    `json:"software_version,omitempty"`
	KeyFingerprint  string    `json:"key_fingerprint,omitempty"`
	Reachable       bool      `json:"reachable"`
	FirstSeen       time.Time `json:"first_seen"`
	LastSeen        time.Time `json:"last_seen"`
}

// Public room listed in a homeserver's public room directory.
//
// Docs:
// - https://spec.matrix.org/v1.19/client-server-api/#get_matrixclientv3publicrooms (room data)
// - https://spec.matrix.org/v1.19/appendices/#room-ids (origin/alias)
// - https://spec.matrix.org/v1.19/appendices/#room-aliases (origin/alias)
type Room struct {
	RoomID           string `json:"room_id"`
	OriginServer     string `json:"origin_server"`
	Name             string `json:"name,omitempty"`
	Topic            string `json:"topic,omitempty"`
	CanonicalAlias   string `json:"canonical_alias,omitempty"`
	AliasServer      string `json:"alias_server,omitempty"`
	NumJoinedMembers int    `json:"num_joined_members"`
	WorldReadable    bool   `json:"world_readable"`
	GuestCanJoin     bool   `json:"guest_can_join"`
	JoinRule         string `json:"join_rule,omitempty"`
	RoomType         string `json:"room_type,omitempty"`
	AvatarURL        string `json:"avatar_url,omitempty"`
}

// Edge between SourceServer (A) and TargetServer (B) where server A's room directory contains a
// room owned/homed on server B. Discovered by querying server A.
type Edge struct {
	SourceServer string    `json:"source_server"`
	TargetServer string    `json:"target_server"`
	RoomID       string    `json:"room_id"`
	SeenAt       time.Time `json:"seen_at"`
}

// Extracts the server name (anything after the first colon). Identifiers can be "!abc:example.org",
// "@user:example.org", etc. The localpart never contains ':', so splitting on the first instance
// supports IPv6 literals in the server portion too.
func serverNameFromID(id string) string {
	_, server, found := strings.Cut(id, ":")
	if !found {
		return ""
	}
	return server
}
