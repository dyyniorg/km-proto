package main

import (
	"context"
	"flag"
	"log"
	"os"
	"strings"
	"time"

	kmproto "github.com/dyyniorg/km-proto"
)

func main() {
	var (
		seedsFlag     = flag.String("seeds", "", "comma-separated seed server names (empty = defaults)")
		limit         = flag.Int("limit", 0, "publicRooms page size (0 = default 100)")
		workers       = flag.Int("workers", 0, "crawl concurrency (0 = default 8)")
		maxServers    = flag.Int("max-servers", 0, "stop after N servers (0 = unlimited)")
		summaryPeriod = flag.Duration("summary-period", 0, "progress-reporting interval (0 = default 15m)")
		timeout       = flag.Duration("timeout", 5*time.Minute, "crawl timeout")
		out           = flag.String("out", "", "output file (empty = stdout)")
	)
	flag.Parse()

	client := kmproto.NewClient("km_proto/demo")
	store := kmproto.NewJSONStore()
	logger := log.New(os.Stderr, "km_proto_demo: ", log.LstdFlags)

	var seeds []string
	// if nil seeds, NewCrawler falls back to using defaults
	if *seedsFlag != "" {
		for _, s := range strings.Split(*seedsFlag, ",") {
			if s = strings.TrimSpace(s); s != "" {
				seeds = append(seeds, s)
			}
		}
	}

	crawler := kmproto.NewCrawler(client, store, logger, seeds, *limit, *workers, *maxServers, *summaryPeriod)
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	stats, err := crawler.Crawl(ctx)
	if err != nil && err != context.DeadlineExceeded {
		logger.Fatalf("crawl failed: %v", err)
	}
	logger.Printf("done: servers=%d rooms=%d edges=%d elapsed=%s",
		stats.ServersFound, stats.RoomsFound, stats.EdgesFound, stats.Elapsed.Round(time.Second))

	w := os.Stdout
	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			logger.Fatalf("opening output: %v", err)
		}
		defer f.Close()
		w = f
	}
	logger.Printf("dumping %d servers, %d rooms, and %d edges...", stats.ServersFound, stats.RoomsFound, stats.EdgesFound)
	if err := store.Dump(w); err != nil {
		logger.Fatalf("dumping: %v", err)
	}
}
