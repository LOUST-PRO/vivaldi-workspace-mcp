# Go Concurrency Notes

This document captures the Go-specific concurrency choices and one
known SDK quirk that anyone reading the source should know about. It is
intended for reviewers who want to understand the threading model and
for maintainers who will extend the tool surface.

## Threading model

The server is a single process. It uses two kinds of goroutines:

1. **The MCP SDK's dispatcher goroutine** — one per in-flight request.
   Created by `server.ServeStdio` inside `mark3labs/mcp-go`. Each
   goroutine reads a JSON-RPC frame from stdin, parses it, dispatches
   to the registered handler, and writes the result back to stdout.
2. **The launcher goroutine for `cmd.Wait()`** — created in
   `pkg/vivaldi/launcher.go` to wait for the spawned `vivaldi` process
   to exit without blocking the tool handler. The handler selects on
   `waitDone` vs `ctx.Done()` to honor the launch timeout.

There is no global mutex, no shared mutable state across tool calls,
and no explicit goroutine for periodic work. Every tool handler is
expected to be **safe for concurrent invocation** because the SDK does
not serialize requests by default.

### Why no shared state needs locking

- **Profile reads** use `os.ReadFile` per call. Two concurrent
  `list_workspaces` calls each read `Preferences` independently.
- **HTML export** writes to a caller-supplied path. Two concurrent
  exports to the same path would race, but each call has its own
  CreateTemp + Rename so the worst case is one of them lands first
  and the second overwrites atomically — no torn file.
- **Snapshot save** writes to a `<snapshot_id>/snapshot.json` path
  determined by caller-supplied label. Two saves with the same label
  race on `mkdir`; the loser gets `os.MkdirAll` returning nil (idempotent)
  and the atomic rename then wins for last-writer. This is the
  expected behavior — last call wins, no torn file.
- **`launch_tabs`** spawns an independent `exec.Cmd` per call. The
  Vivaldi instance serializes URL queueing through its own
  SingleInstanceLock (`~/.config/vivaldi/SingletonLock`), so two
  concurrent `vivaldi` processes cannot both think they own the profile.

## The stdio transport's parallel-dispatch quirk

The `mark3labs/mcp-go` stdio transport (as of v0.57.0) does **not**
serialize requests. If you send two `tools/call` frames in quick
succession on stdin, they may be dispatched to handlers concurrently.
This shows up most clearly with the save → list snapshot pattern:

```
[client sends]
  tools/call save_workspace_snapshot(...)   (id: 7)
  tools/call list_snapshots(limit=5)        (id: 8)

[handlers may run in parallel]
  save handler:   profile + tabs read → atomic write
  list handler:   ReadDir on base dir   → []

[client receives]
  id:7 result {snapshot_id: "smoke-test-snap", path: ..., bytes: 4231}
  id:8 result {count: 0, snapshots: []}    ← empty because list raced save
```

The smoke test (`scripts/smoke.sh`) works around this by:

1. Sleeping 1 second between the save and the first list, which is
   enough for the write + fsync + rename to settle in practice.
2. Issuing a second list call (id: 9) after another short delay and
   asserting the snapshot is visible there. The "saw 0 on first try,
   found 1 on retry" outcome is accepted as expected behavior and
   logged as `[smoke] first try saw 0 (expected race)`.

This is **not** a server bug — concurrent calls are correctly handled,
and `list_snapshots` simply does not block waiting for in-flight
writes. If a future user wants "read your own write" semantics, they
should send a single batched request through their MCP client (one
`tools/call` after the previous one returns).

## `signal.NotifyContext` and graceful shutdown

`main.go` wraps `context.Background()` with `signal.NotifyContext` for
SIGINT and SIGTERM. The returned cancel function is deferred so the
context is cancelled on either signal. We do **not** currently pass
that context to `server.ServeStdio` because the SDK does not yet
accept a context on `ServeStdio`. Instead, the running process exits
when the host closes stdin (the typical MCP client shutdown flow).
The signal context is therefore mostly defensive — if a future SDK
release adds context support, the wiring is already in place.

## Atomic file writes

The pattern in `pkg/vivaldi/filex/atomic.go` is the standard
"write to temp, fsync, rename" sequence:

```go
tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-")
// … write tmp …
if err := tmp.Sync(); err != nil { … }    // fsync the data
if err := os.Rename(tmp.Name(), path); …   // atomic on POSIX
```

The order matters: `Sync` (fsync) flushes the file's data to disk
before `Rename` publishes the new file to the rest of the filesystem.
On a crash:

- Before `Sync`: the temp file may be partially written, but the
  final destination file is still the previous version. Cleanup of the
  orphan temp file is the OS's job on next boot.
- Between `Sync` and `Rename`: the new data is on disk but not yet
  visible at `path`. The destination is still the previous version.
- After `Rename`: the destination file is the new content, atomically.

This is why `export_workspace_html` can claim `atomic: true` in its
response and why `save_workspace_snapshot` can survive a kill -9
without leaving a half-written snapshot in the directory.

## Why we use Go's standard library only (plus 3 deps)

`go.mod` has three direct dependencies:

- `github.com/louzt/vivaldi-workspace-mcp/pkg/vivaldi` — that's us.
- `github.com/mark3labs/mcp-go/mcp` — the MCP SDK.
- `github.com/mark3labs/mcp-go/server` — same SDK, server side.

Everything else (`os`, `path/filepath`, `encoding/json`, `regexp`,
`context`, `sync`, `sort`, `strings`, `time`) is from the standard
library. This keeps the binary small (~9 MB statically linked) and
the supply-chain surface narrow. A reviewer can audit the entire
dependency closure in a few minutes.

## Testing the concurrency invariants

`go test ./...` runs unit tests serially by default (Go test runs
within one package serially unless you call `t.Parallel()`). The
concurrency invariants above are therefore not exercised by the test
suite — they are exercised by `scripts/smoke.sh` which sends multiple
JSON-RPC frames back-to-back and observes whether the responses are
well-formed under concurrent handler invocation.

If you change a tool handler to introduce shared state (a global
cache, a pool, a counter), add either:

1. A `t.Parallel()` test that fires N concurrent goroutines at the
   handler, or
2. A smoke.sh extension that pipelines multiple calls of the changed
   tool and asserts the aggregated result.

Either will catch a regression before it ships.
