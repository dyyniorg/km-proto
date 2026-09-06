package kmproto

import (
	"encoding/json"
	"io"
	"sync"
)

/* store interface + JSONStore (dump for debugging/oneshot rounds) */

type Store interface {
	PutServer(Server) error
	PutRoom(Room) error
	PutEdge(Edge) error
}

// Thread-safe in-memory collector for oneshot runs and testing/debugging.
type JSONStore struct {
	mu      sync.Mutex
	servers []Server
	rooms   []Room
	edges   []Edge
}

func NewJSONStore() *JSONStore {
	return &JSONStore{}
}

func (s *JSONStore) PutServer(srv Server) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.servers = append(s.servers, srv)
	return nil
}

func (s *JSONStore) PutRoom(r Room) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rooms = append(s.rooms, r)
	return nil
}

func (s *JSONStore) PutEdge(e Edge) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.edges = append(s.edges, e)
	return nil
}

// record is a JSONL envelope carrying a single typed crawl result.
type record struct {
	Type   string  `json:"type"`
	Server *Server `json:"server,omitempty"`
	Room   *Room   `json:"room,omitempty"`
	Edge   *Edge   `json:"edge,omitempty"`
}

// Writes all buffered records as JSONL in deterministic order: servers, rooms, and edges. Each
// line is a JSON object with a "type" field and the record body under the corresponding key.
func (s *JSONStore) Dump(w io.Writer) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	enc := json.NewEncoder(w)
	for i := range s.servers {
		if err := enc.Encode(record{Type: "server", Server: &s.servers[i]}); err != nil {
			return err
		}
	}
	for i := range s.rooms {
		if err := enc.Encode(record{Type: "room", Room: &s.rooms[i]}); err != nil {
			return err
		}
	}
	for i := range s.edges {
		if err := enc.Encode(record{Type: "edge", Edge: &s.edges[i]}); err != nil {
			return err
		}
	}
	return nil
}
