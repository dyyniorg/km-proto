# km-proto

Matrix protocol (v1.19) crawler library utilizing the [server-server](https://spec.matrix.org/v1.19/server-server-api/) and [client-server](https://spec.matrix.org/v1.19/client-server-api/) APIs in tier-1 fashion (i.e. no authentication or object signing of any kind required).

- Collects homeservers, public rooms, and maps edges/topology between federated homeservers
    - Server data: domain (`server_name`), host/port (`resolved_host`), server software and versioning (`software_name`, `software_version`), SHA256 of the server's signing keys (`key_fingerprint`), reachability status, & crawl-related timestamps
    - Room data: unique identifier (`room_id`), room's host homeserver (`origin_server`), human-readable name (`name`), topic text (`topic`), primary alias (`canonical_alias`), homeserver owning the alias (`alias_server`), amount of joined users (`num_joined_members`), whether the room is viewable without joining (`world_readable`), whether guests can join (`guest_can_join`), join policy (`join_rule`), room type (`room_type`), & room avatar location (`avatar_url`)
    - Edges map a directed connection from `source_server` to `target_server` (the room's origin), identifying the room (`room_id`) through which the connection was discovered
- Crawls progress as breadth-first search, allowing both context-based time limits & maximum server count limit
- Provides a clean `Store` interface for database connectors
    - Data structure can be mapped 1:1 onto SQLite schema (`servers`/`rooms`/`edges`) and supports building directed graphs of the federation topology

## Installation

```shell
go get github.com/dyyniorg/km-proto@latest
```

## Example

```go
client := kmproto.NewClient("")
store := kmproto.NewJSONStore()
logger := log.New(os.Stderr, "km-proto: ", log.LstdFlags)

crawler := kmproto.NewCrawler(client, store, logger, nil, 0, 0, 0, 0)

ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()

stats, err := crawler.Crawl(ctx)
if err != nil {
    logger.Fatalf("crawl: %v", err)
}
logger.Printf("done: %d servers, %d rooms, %d edges", stats.ServersFound, stats.RoomsFound, stats.EdgesFound)

if err := store.Dump(os.Stdout); err != nil {
    logger.Fatalf("dump: %v", err)
}
```

A complete runnable program is available under [`examples/oneshot`](examples/oneshot).

