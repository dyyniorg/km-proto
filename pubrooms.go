package kmproto

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
)

/*
	NOTE:

	Homeservers *may* publish a room directory to allow any user to discover rooms. A room can have
	either public or private visibility, which determines whether it's listed in the directory. In
	addition to that the homeservers may also perform other kinds of filtering, so it's good to note
	that the crawler only discovers *published* rooms.

	Additionally we're using the C-S API instead of S-S due to the former being unauthenticated.
*/

var (
	errNoDirectory = errors.New("server does not publish a room directory")
)

// Docs: https://spec.matrix.org/v1.19/client-server-api/#get_matrixclientv3publicrooms
type publicRoomsResponse struct {
	Chunk                  []publicRoom `json:"chunk"`
	NextBatch              string       `json:"next_batch"`
	PrevBatch              string       `json:"prev_batch"`
	TotalRoomCountEstimate *int         `json:"total_room_count_estimate"` // (if the server has an estimate)
}

// Chunked entry, before enrichment into model.Room.
type publicRoom struct {
	AvatarURL        string `json:"avatar_url"`
	CanonicalAlias   string `json:"canonical_alias"`
	GuestCanJoin     bool   `json:"guest_can_join"`
	JoinRule         string `json:"join_rule"`
	Name             string `json:"name"`
	NumJoinedMembers int    `json:"num_joined_members"`
	RoomID           string `json:"room_id"`
	RoomType         string `json:"room_type"`
	Topic            string `json:"topic"`
	WorldReadable    bool   `json:"world_readable"`
}

// Fetches one page of the server's public room directory, returning Room records enriched with
// origin/alias server names, and the cursor for the next page (or "" when exhausted). Uses the
// unauthenticated client-server API endpoint.
func (c *Client) PublicRooms(ctx context.Context, name string, limit int, since string) ([]Room, string, error) {
	base, err := c.clientBase(ctx, name)
	if err != nil {
		return nil, "", err
	}

	qp := url.Values{}
	qp.Set("limit", strconv.Itoa(limit))
	if since != "" {
		qp.Set("since", since)
	}

	target := base + "/_matrix/client/v3/publicRooms?" + qp.Encode()

	var resp publicRoomsResponse
	if err := c.GetJSON(ctx, target, "", "", &resp); err != nil {
		var he *HTTPError
		if errors.As(err, &he) && (he.StatusCode == http.StatusNotFound || he.StatusCode == http.StatusMethodNotAllowed) {
			return nil, "", errNoDirectory
		}
		return nil, "", err
	}

	rooms := make([]Room, 0, len(resp.Chunk))
	for _, p := range resp.Chunk {
		rooms = append(rooms, Room{
			RoomID:           p.RoomID,
			OriginServer:     serverNameFromID(p.RoomID),
			Name:             p.Name,
			Topic:            p.Topic,
			CanonicalAlias:   p.CanonicalAlias,
			AliasServer:      serverNameFromID(p.CanonicalAlias),
			NumJoinedMembers: p.NumJoinedMembers,
			WorldReadable:    p.WorldReadable,
			GuestCanJoin:     p.GuestCanJoin,
			JoinRule:         p.JoinRule,
			RoomType:         p.RoomType,
			AvatarURL:        p.AvatarURL,
		})
	}

	return rooms, resp.NextBatch, nil

}
