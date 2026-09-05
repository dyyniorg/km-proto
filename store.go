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

// Writes all buffered records as JSONL in deterministic order: servers, rooms, and edges. Each
// line is a JSON object with a "type" field and the record body under the corresponding key.
func (s *JSONStore) Dump(w io.Writer) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	enc := json.NewEncoder(w)
	for _, srv := range s.servers {
		if err := enc.Encode(map[string]any{"type": "server", "server": srv}); err != nil {
			return err
		}
	}
	for _, r := range s.rooms {
		if err := enc.Encode(map[string]any{"type": "room", "room": r}); err != nil {
			return err
		}
	}
	for _, e := range s.edges {
		if err := enc.Encode(map[string]any{"type": "edge", "edge": e}); err != nil {
			return err
		}
	}
	return nil
}
