# Snapshots

The snapshot subsystem lets you capture the current state of your
Vivaldi profile (workspace configuration + tab URLs) to a single
JSON file on disk, and later restore by re-launching the captured
URLs in Vivaldi. Snapshots are intentionally simple: one JSON file
per snapshot, no database, no compression, no encryption.

## File layout

By default, snapshots live under
`$XDG_DATA_HOME/vivaldi-workspace-mcp/snapshots/<snapshot_id>/`:

```
$XDG_DATA_HOME/vivaldi-workspace-mcp/snapshots/
├── 2026-08-04T14-32-08Z/
│   └── snapshot.json
├── pre-launch-cleanup/
│   └── snapshot.json
└── weekend-research-2026-08-02/
    └── snapshot.json
```

`<snapshot_id>` is one of:

- The `label` argument you passed to `save_workspace_snapshot`,
  sanitized to filename-safe characters (`[A-Za-z0-9._-]`).
- A UTC timestamp `YYYY-MM-DDTHH-MM-SSZ` if you did not pass a label.

To override the base directory (used by smoke tests), set
`VIVALDI_SNAPSHOT_DIR`. **This is intended for tests and CI**;
leaving it unset on a normal install is correct.

## The `snapshot.json` schema

```json
{
  "snapshot_id": "pre-launch-cleanup",
  "saved_at": "2026-08-04T14:32:08.123456789Z",
  "version": "1.1.0",
  "profile_path": "/home/lou/.config/vivaldi/Default",
  "workspaces": [
    {
      "workspace_id": "1",
      "workspace_name": "Developer",
      "count": 47,
      "tabs": [
        { "url": "https://github.com/...", "domain": "github.com" },
        …
      ]
    },
    …
  ],
  "totals": { "workspaces": 6, "urls": 312 }
}
```

Field reference:

- `snapshot_id`: the sanitized label, or the UTC timestamp. Also the
  directory name.
- `saved_at`: RFC3339Nano UTC timestamp of when the snapshot was
  written. Useful for human display, not for ordering (use
  `snapshot_id` for that — it sorts lexicographically the same as
  chronologically when timestamps are in `YYYY-MM-DDTHH-MM-SSZ` form).
- `version`: the schema version. Matches the MCP server version when
  the snapshot was written. See "Schema stability" below.
- `profile_path`: the absolute path of the Vivaldi profile that was
  read. Useful for verifying the snapshot was taken against the
  profile you expect.
- `workspaces[]`: one entry per workspace, with the tabs that were
  assigned to it via the round-robin heuristic (see below).
- `totals`: cheap counters so `list_snapshots` can render without
  parsing the full `tabs` array.

### Why tabs are per-workspace, not per-tab

Vivaldi's binary session files (`Sessions/Tabs_*`) do not embed the
workspace ID for each tab. The snapshot approximates the mapping by
distributing the flat tab list across the configured workspaces in
round-robin order (`buildWorkspaceTabs` in
`pkg/vivaldi/snapshot.go`). This is a heuristic — the per-workspace
counts are best-effort.

For exact per-tab workspace attribution, the live `Preferences` file
contains a `vivaldi.sessions` array (added in Vivaldi 5+) that maps
session_id → workspace_id. Parsing that is out of scope for this
server's read-only profile contract.

## Schema stability contract

The on-disk schema is **additive-only** across versions:

- Future versions of this server MUST be able to read v1.x records
  without code changes.
- To evolve the schema, **add new fields** with JSON tags. Never
  rename or remove existing fields, and never change the JSON
  representation of an existing field's type.
- Bump the `version` field only when consumers should re-read or
  when an incompatible interpretation would otherwise result.

In practice this means a snapshot written by v1.1.0 will be readable
by every future v1.x release. A v2.x release (if we ever ship one)
will still read v1.x records but may write v2.x records with
additional fields.

## Restoring a snapshot

`restore_workspace_snapshot` loads the snapshot, flattens the URLs
across the requested workspace(s), and splits them into batches of
**30 URLs per call to `vivaldi`**. The batch size is chosen to
avoid a UI freeze in Vivaldi when 200+ tabs are queued at once.

The restore path does **not** route tabs to specific workspaces —
Vivaldi's CLI accepts a flat URL list and queues them all in the
currently-active window. If you need different tabs in different
workspaces, switch workspaces manually in Vivaldi after the launch.

## Why a snapshot and not the live `Sessions/` files?

Two reasons:

1. **`Sessions/Tabs_*` is a binary format** that requires a Chromium
   session parser to read correctly. Storing as JSON keeps the data
   readable, diffable, and version-controllable.
2. **Snapshots are point-in-time and version-tagged.** The live
   session files are constantly being overwritten as Vivaldi runs,
   which makes them useless for "I want to undo my last hour of
   browsing" use cases.

## Limitations

- A snapshot captures *open and recovered* URLs from the session
  files. It does **not** capture which tabs were grouped in a Tab
  Stack, which window they were in, or their zoom/scroll position.
- The per-workspace distribution is heuristic. If you have specific
  routing requirements, use `restore_workspace_snapshot` with
  `workspace_id` to launch just one workspace's tabs at a time.
- There is no built-in snapshot rotation or retention. If you want
  to keep only the last N snapshots, you can either delete the
  snapshot directories manually or wrap `list_snapshots` in a shell
  one-liner that prunes the oldest entries.

## Listing snapshots

`list_snapshots` returns metadata only — it does not parse the
`tabs` arrays. This keeps the call cheap even when you have hundreds
of saved snapshots:

```json
{
  "base_dir": "/home/lou/.local/share/vivaldi-workspace-mcp/snapshots",
  "count": 3,
  "snapshots": [
    { "snapshot_id": "weekend-research-2026-08-02", "saved_at": "…", "version": "1.1.0", … },
    { "snapshot_id": "pre-launch-cleanup",            "saved_at": "…", "version": "1.1.0", … },
    { "snapshot_id": "2026-08-04T14-32-08Z",          "saved_at": "…", "version": "1.1.0", … }
  ]
}
```

Order is newest-first. The optional `limit` argument caps the
returned count without affecting the on-disk files.

## Atomic write

`save_workspace_snapshot` writes `snapshot.json` via
`filex.WriteFileAtomic`. If the process is killed mid-write, the
destination file is either the previous version or does not exist
yet — never a half-written JSON document. See
[security-model.md](security-model.md#atomic-writes-prevent-corruption-on-crash)
for the full pattern.
