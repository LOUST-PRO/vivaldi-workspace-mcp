# Security Model

`vivaldi-workspace-mcp` is a **local-only** tool. It runs as a
subprocess of your MCP client on your Linux desktop, reads your local
Vivaldi profile, writes to a few well-defined local paths, and spawns
`vivaldi` to open URLs. It does not open any TCP port, does not call
home, and does not require root.

This document describes the trust boundary, the data flow across it,
and the design choices we made to keep it small.

## Trust boundary

```
┌─────────────────────────────────────────────────────────────────┐
│ Linux user session (UID = $USER)                                │
│                                                                 │
│  ┌──────────────┐    JSON-RPC    ┌──────────────────────┐       │
│  │  MCP client  │ ─────────────► │ vivaldi-workspace-mcp│       │
│  │  (trusted    │ ◄───────────── │  (this binary, same  │       │
│  │   by user)   │     stdio      │   UID, no caps)      │       │
│  └──────────────┘                └──────────────────────┘       │
│                                           │                      │
│                          reads/writes     │  exec                │
│                                           ▼                      │
│                       ~/.config/vivaldi/Default/                 │
│                       ~/.local/share/vivaldi-workspace-mcp/      │
│                       <caller-supplied output path>              │
│                       vivaldi (chromium binary)                  │
└─────────────────────────────────────────────────────────────────┘

Out of scope (the server NEVER does these):
  - Open TCP / UDP sockets
  - Reach an external host on its own
  - Read other users' profiles (no privilege escalation)
  - Modify the live Vivaldi profile while Vivaldi is running
    (read-only on Preferences; atomic write on snapshots/HTML)
```

## What each tool can and cannot do

The MCP 2025-06-18 `ToolAnnotations` block travels with every tool
descriptor so the client can render "may modify state" warnings before
invocation. Below is the full table:

| Tool | readOnly | destructive | idempotent | open-world | Effective capability |
|---|---|---|---|---|---|
| `list_workspaces` | ✅ | ❌ | ✅ | ❌ | Reads `Preferences` JSON. |
| `list_workspace_tabs` | ✅ | ❌ | ✅ | ❌ | Reads `Sessions/Tabs_*` binary files. |
| `export_workspace_html` | ❌ | ❌ | ✅ | ❌ | Writes a single file at caller-supplied path via atomic temp+rename. |
| `launch_tabs` | ❌ | ❌ | ✅ | ❌ | Spawns `vivaldi` with validated URLs. |
| `save_workspace_snapshot` | ❌ | ❌ | ✅ | ❌ | Writes one snapshot JSON file under `$XDG_DATA_HOME` (or `VIVALDI_SNAPSHOT_DIR` if set). |
| `restore_workspace_snapshot` | ❌ | ❌ | ✅¹ | ❌ | Reads a snapshot file, then spawns `vivaldi` with its URLs in batches of 30. |
| `list_snapshots` | ✅ | ❌ | ✅ | ❌ | Reads metadata from snapshot directory. |

¹ `restore_workspace_snapshot` is marked idempotent because re-running
it with the same `snapshot_id` produces the same URL queue. The browser
may end up with duplicate tabs if Vivaldi's SingleInstanceLock queues
the same URL twice — that is the caller's responsibility, not a server
defect.

## Why we restrict URL schemes

`launch_tabs` accepts a comma-separated list of URLs from the caller.
We classify them in `splitValidURLs` (`pkg/vivaldi/launcher.go`):

- **Accepted**: starts with `http://` or `https://`, length ≥ 12.
- **Rejected**: anything else. Reported back in the response with the
  reason, **not silently dropped**.

Why we reject `file://`, `javascript:`, and custom schemes:

- **`file://`** lets a model open arbitrary local files, including
  `/etc/passwd` or `~/.ssh/id_rsa`. Most users do not want an AI
  assistant to be able to do this without an explicit prompt.
- **`javascript:`** lets a model execute arbitrary script in Vivaldi's
  privileged context. The prompt-injection risk is unbounded.
- **Custom schemes** (e.g. `vivaldi://settings`) are navigation
  attempts to internal pages that can change browser settings.

The filter is intentionally narrow — `http(s)` only — because that is
what 99% of MCP clients actually want from a "launch these tabs in my
browser" tool, and the rejected URLs are surfaced so the user can see
when the model tried to step outside the boundary.

## Atomic writes prevent corruption on crash

Both `export_workspace_html` and `save_workspace_snapshot` write
through `pkg/vivaldi/filex.WriteFileAtomic`, which:

1. Creates `<path>.tmp-XXXXXX` in the same directory.
2. Writes the full payload.
3. `fsync()`s the file to flush to disk.
4. Renames the temp file over the destination.

On a `kill -9` between step 2 and step 4, the destination file is
**still the previous version** — the OS reclaims the orphan temp file
on next boot. The user never sees a half-written report or a
half-written snapshot.

This is the same pattern that Git uses for object writes, that
SQLite uses for journal commits, and that most production tools use
for "I cannot afford a torn write here."

## What the server does NOT do

- Does not run as root. If your Vivaldi profile lives in a directory
  only root can read, the server will fail with a clear permission
  error rather than silently falling back.
- Does not setuid or drop privileges.
- Does not modify the live `Preferences` or `Sessions/` files. It only
  reads them. (The `SingleInstanceLock` would fight us anyway if
  Vivaldi is running, but we don't try.)
- Does not log full URLs anywhere by default. The logger writes to
  stderr only, with severity levels, and never includes request bodies.
- Does not implement any caching of profile data. Every read goes back
  to disk. This is intentional: a stale cache could mislead the user
  about their actual browser state.

## Supply chain

Three direct dependencies (`go.mod`):

```
github.com/louzt/vivaldi-workspace-mcp   (this repo's own pkg/)
github.com/mark3labs/mcp-go              (the MCP SDK; v0.57.0+)
```

Everything else is the Go standard library. Run `go mod tidy && go
mod verify` after any update to confirm the lockfile is consistent
with `go.sum`.

## Reporting a vulnerability

Please open a GitHub issue tagged `security` or email the maintainer
directly (see the repo's `CODEOWNERS` once the project grows).
This project is a small surface area — there is no formal disclosure
policy yet, but the maintainer aims to triage within 48h.
