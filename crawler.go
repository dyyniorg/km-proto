package kmproto

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"
)

var (
	// Well-known homeservers used when no seeds are given by the user.
	DefaultSeeds = []string{
		"matrix.org",
		// TODO: fill in 2-3 more items here, preferably quite large homeservers
	}
)

type Crawler struct {
	client   *Client
	store    Store
	resolver *Resolver
	seeds    []string

	limit      int
	workers    int
	maxServers int // 0 = unlimited

	mu             sync.Mutex
	seenServers    map[string]bool
	seenRooms      map[string]bool
	seenEdges      map[string]bool
	queue          []string
	waiter         *sync.Cond
	active         int // in-flight server visits
	serversVisited int // compared against the optional maxServers budget

	logger *log.Logger
}

func NewCrawler(client *Client, store Store, logger *log.Logger, seeds []string, limit, workers, maxServers int) *Crawler {
	if logger == nil {
		logger = log.Default()
	}
	if client == nil {
		client = NewClient("")
	}
	if store == nil {
		store = NewJSONStore()
	}
	if len(seeds) == 0 {
		seeds = DefaultSeeds
	}
	if limit <= 0 {
		limit = 100
	}
	if workers <= 0 {
		workers = 8
	}
	c := &Crawler{
		client:      client,
		store:       store,
		resolver:    NewResolver(client),
		seeds:       seeds,
		limit:       limit,
		workers:     workers,
		maxServers:  maxServers,
		seenServers: make(map[string]bool),
		seenRooms:   make(map[string]bool),
		seenEdges:   make(map[string]bool),
		logger:      logger,
	}
	c.waiter = sync.NewCond(&c.mu)
	return c
}

// Breadth-first walk of the federation network, starting from the defined/default seed servers.
// Context can be set to cancel the crawl early (using a timeout) or alternatively maxServers can
// be set (via NewCrawler) to bound the max. number of servers visited by the crawler.
func (c *Crawler) Crawl(ctx context.Context) error {
	c.mu.Lock()
	for _, s := range c.seeds {
		c.enqueueLocked(s)
	}
	c.mu.Unlock()

	work := make(chan string)
	var wg sync.WaitGroup

	for i := 0; i < c.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for name := range work {
				c.crawlServer(ctx, name)
			}
		}()
	}

	go func() {
		defer close(work)
		for {
			name, ok := c.dequeue()
			if !ok {
				return
			}
			select {
			case work <- name:
			case <-ctx.Done():
				return
			}
		}
	}()

	wg.Wait()
	return nil
}

// Blocks until a server name is available or the queue is empty and no visits are in-flight (i.e.
// the crawl is finished). Returns the next server name, incrementing active count, or ok=false
// when the crawl is complete.
func (c *Crawler) dequeue() (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for len(c.queue) == 0 && c.active > 0 {
		c.waiter.Wait()
	}
	if len(c.queue) == 0 {
		return "", false // queue drained and all active done
	}
	name := c.queue[0]
	c.queue = c.queue[1:]
	c.active++
	return name, true
}

// Visits a single homeserver, resolves its name, fingerprints it, drains the server's published
// room directory page by page, and records each room plus an edge to the room's origin server,
// enqueuing newly seen servers into the BFS queue. Defers 'markDone' call so active/
// serversVisited are maintained on every exit path regardless of the outcome.
func (c *Crawler) crawlServer(ctx context.Context, name string) {
	defer c.markDone()

	rs, err := c.resolver.Resolve(ctx, name)
	if err != nil {
		c.store.PutServer(Server{ServerName: name, Reachable: false, FirstSeen: time.Now(), LastSeen: time.Now()})
		return
	}

	srv, _ := c.client.Fingerprint(ctx, rs)
	c.store.PutServer(srv)

	since := ""
	for {
		if !c.budgetAllows() || ctx.Err() != nil {
			return
		}
		rooms, next, err := c.client.PublicRooms(ctx, rs, c.limit, since)
		if err != nil {
			if !errors.Is(err, errNoDirectory) {
				c.logger.Printf("pubrooms %q: %v", name, err)
			}
			return
		}
		for _, r := range rooms {
			c.maybePutRoom(r)
			if r.OriginServer != "" && r.OriginServer != name {
				c.maybePutEdge(name, r)
				c.enqueueIfNew(r.OriginServer)
			}
			if r.AliasServer != "" && r.AliasServer != name {
				c.enqueueIfNew(r.AliasServer)
			}
		}
		if next == "" {
			return
		}
		since = next
	}
}

func (c *Crawler) markDone() {
	c.mu.Lock()
	c.active--
	c.serversVisited++
	c.waiter.Broadcast()
	c.mu.Unlock()
}

func (c *Crawler) budgetAllows() bool {
	// NOTE: briefly overshoots max. amount due to in-flight requests completing before cutting power
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.maxServers == 0 || c.serversVisited < c.maxServers
}

func (c *Crawler) enqueueLocked(name string) {
	if name == "" || c.seenServers[name] {
		return
	}
	c.seenServers[name] = true
	c.queue = append(c.queue, name)
}

func (c *Crawler) enqueueIfNew(name string) {
	c.mu.Lock()
	c.enqueueLocked(name)
	c.waiter.Broadcast()
	c.mu.Unlock()
}

func (c *Crawler) maybePutRoom(r Room) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.seenRooms[r.RoomID] {
		return
	}
	c.seenRooms[r.RoomID] = true
	c.store.PutRoom(r)
}

func (c *Crawler) maybePutEdge(source string, r Room) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := source + "\x00" + r.RoomID
	if c.seenEdges[key] {
		return
	}
	c.seenEdges[key] = true
	c.store.PutEdge(Edge{
		SourceServer: source,
		TargetServer: r.OriginServer,
		RoomID:       r.RoomID,
		SeenAt:       time.Now(),
	})
}
