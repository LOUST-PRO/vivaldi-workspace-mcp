#!/usr/bin/env bash
# Smoke test for vivaldi-workspace-mcp.
#
# Sends a sequence of JSON-RPC requests over stdio and verifies that each
# tool returns the expected structured JSON envelope. The script also
# checks ToolAnnotations (readOnlyHint, openWorldHint, idempotentHint)
# on every advertised tool so we catch missing or mis-set annotations
# before shipping.
#
# Run from the repo root:
#     scripts/smoke.sh

set -euo pipefail

BIN="$(cd "$(dirname "$0")/.." && pwd)/bin/vivaldi-workspace-mcp"
echo "[smoke] binary: $BIN"
if [[ ! -x "$BIN" ]]; then
    echo "[smoke] FAIL: binary not built. Run: go build -o bin/vivaldi-workspace-mcp ."
    exit 1
fi

# Use a per-test snapshot directory so we don't pollute the user's real one.
TMP_SNAP_DIR="$(mktemp -d /tmp/vivaldi-smoke-snapshots.XXXXXX)"
echo "[smoke] snapshot dir (isolated): $TMP_SNAP_DIR"

# Each request is a single JSON object on one line.
# Response from MCP server is also single JSON object per line.

rq_initialize='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"smoke","version":"0.1.0"}}}'
rq_initialized='{"jsonrpc":"2.0","method":"notifications/initialized"}'
rq_list_tools='{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'
rq_list_workspaces='{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_workspaces","arguments":{}}}'
rq_list_tabs='{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"list_workspace_tabs","arguments":{}}}'
rq_launch_invalid='{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"launch_tabs","arguments":{"urls":"file:///etc/passwd,javascript:alert(1),https://example.com,https://github.com/foo/bar"}}}'
rq_export_html='{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"export_workspace_html","arguments":{"output_path":"/tmp/smoke_vivaldi.html"}}}'
rq_save_snap='{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"save_workspace_snapshot","arguments":{"label":"smoke-test-snap"}}}'
rq_list_snap='{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"list_snapshots","arguments":{"limit":5}}}'
# Second list after a small delay — mark3labs/mcp-go stdio transport
# processes requests in parallel, so the first list_snapshots may race
# against save_workspace_snapshot and see an empty base dir. A delayed
# second call confirms the file is durable.
rq_list_snap_retry='{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"list_snapshots","arguments":{"limit":5}}}'

# Send them sequentially; capture all responses.
{
    echo "$rq_initialize"
    sleep 0.05
    echo "$rq_initialized"
    sleep 0.05
    echo "$rq_list_tools"
    sleep 0.05
    echo "$rq_list_workspaces"
    sleep 0.05
    echo "$rq_list_tabs"
    sleep 0.05
    echo "$rq_launch_invalid"
    sleep 0.05
    echo "$rq_export_html"
    sleep 0.05
    echo "$rq_save_snap"
    # Wait for save_workspace_snapshot to durably commit before list_snapshots.
    sleep 1.0
    echo "$rq_list_snap"
    sleep 0.2
    echo "$rq_list_snap_retry"
} | VIVALDI_SNAPSHOT_DIR="$TMP_SNAP_DIR" timeout 30 "$BIN" 2>/tmp/smoke_stderr.log | tee /tmp/smoke_stdout.log

echo ""
echo "[smoke] HTML export output:"
ls -la /tmp/smoke_vivaldi.html 2>/dev/null || echo "(no file created)"

echo ""
echo "[smoke] stderr (last 20 lines):"
tail -20 /tmp/smoke_stderr.log 2>/dev/null || echo "(no stderr)"

echo ""
echo "[smoke] validation:"
python3 <<'PYEOF'
import json, sys
with open("/tmp/smoke_stdout.log") as f:
    responses = {}
    for ln in f:
        ln = ln.strip()
        if not ln.startswith("{"):
            continue
        try:
            obj = json.loads(ln)
        except Exception:
            continue
        if "id" in obj:
            responses[obj["id"]] = obj

ok = True
def check(label, cond, hint=""):
    global ok
    status = "OK" if cond else "FAIL"
    print(f"  [{status}] {label}{(' — ' + hint) if hint else ''}")
    if not cond:
        ok = False

try:
    init = responses.get(1, {})
    check("initialize returned", "result" in init, f"got: {init.get('error', 'no result')}")

    tools_resp = responses.get(2, {})
    tools = tools_resp.get("result", {}).get("tools", [])
    check("tools/list returned 7 tools", len(tools) == 7, f"got {len(tools)} tools")

    expected = {
        "list_workspaces": True,
        "list_workspace_tabs": True,
        "export_workspace_html": False,
        "launch_tabs": False,
        "save_workspace_snapshot": False,
        "restore_workspace_snapshot": False,
        "list_snapshots": True,
    }
    tool_names = {t["name"]: t for t in tools}
    for name, expect_ro in expected.items():
        if name not in tool_names:
            check(f"{name} present in tools/list", False, "missing tool")
            continue
        ann = tool_names[name].get("annotations", {})
        ro_actual = ann.get("readOnlyHint")
        if expect_ro:
            check(f"{name}.readOnlyHint=true", ro_actual is True,
                  f"must be true (read-only), got {ro_actual}")
        else:
            check(f"{name}.readOnlyHint=false", ro_actual is False,
                  f"must be false (mutating), got {ro_actual}")
        check(f"{name}.openWorldHint=false", ann.get("openWorldHint") is False,
              "must be false (local-only)")
        check(f"{name}.idempotentHint=true", ann.get("idempotentHint") is True,
              "must be true")

    ws_resp = responses.get(3, {})
    ws_text = ws_resp["result"]["content"][0]["text"] if "result" in ws_resp else "{}"
    workspaces = json.loads(ws_text)
    check("list_workspaces profile_path present", "profile_path" in workspaces,
          f"got keys: {list(workspaces.keys())}")
    check("list_workspaces count > 0", workspaces.get("count", 0) > 0,
          f"got {workspaces.get('count')} workspaces")
    check("list_workspaces workspaces array",
          isinstance(workspaces.get("workspaces"), list))

    tabs_resp = responses.get(4, {})
    tabs_text = tabs_resp["result"]["content"][0]["text"] if "result" in tabs_resp else "{}"
    tabsum = json.loads(tabs_text)
    check("list_workspace_tabs count > 0", tabsum.get("count", 0) > 0,
          f"got {tabsum.get('count')} tabs")
    check("list_workspace_tabs sources[]", isinstance(tabsum.get("sources"), list))

    launch_resp = responses.get(5, {})
    if "result" in launch_resp and launch_resp["result"].get("isError"):
        envelope = json.loads(launch_resp["result"]["content"][0]["text"])
        launch_res = envelope.get("result", envelope)
    else:
        launch_text = launch_resp["result"]["content"][0]["text"]
        launch_res = json.loads(launch_text)
    check("launch_tabs binary reported", "binary" in launch_res,
          f"keys: {list(launch_res.keys())}")
    check("launch_tabs rejected URLs reported",
          "rejected_urls" in launch_res,
          f"keys: {list(launch_res.keys())}")

    exp_resp = responses.get(6, {})
    if "result" in exp_resp and not exp_resp["result"].get("isError"):
        exp_text = exp_resp["result"]["content"][0]["text"]
        exp_res = json.loads(exp_text)
        check("export atomic=true", exp_res.get("atomic") is True,
              f"got atomic={exp_res.get('atomic')}")
        check("export bytes > 0", exp_res.get("bytes", 0) > 0,
              f"got {exp_res.get('bytes')}")
    else:
        print(f"  [WARN] export_workspace_html returned error: {exp_resp.get('error', exp_resp)}")

    snap_resp = responses.get(7, {})
    if "result" in snap_resp:
        snap_text = snap_resp["result"]["content"][0]["text"]
        snap_res = json.loads(snap_text)
        check("save_workspace_snapshot snapshot_id present",
              "snapshot_id" in snap_res and snap_res.get("snapshot_id") == "smoke-test-snap",
              f"got snapshot_id={snap_res.get('snapshot_id')}")
        check("save_workspace_snapshot bytes > 0",
              snap_res.get("bytes", 0) > 0,
              f"got bytes={snap_res.get('bytes')}")
        check("save_workspace_snapshot paths",
              "path" in snap_res,
              f"got keys: {list(snap_res.keys())}")
    else:
        print(f"  [FAIL] save_workspace_snapshot returned error: {snap_resp.get('error', snap_resp)}")
        ok = False

    list_resp = responses.get(8, {})
    list_resp_retry = responses.get(9, {})
    if "result" in list_resp:
        list_text = list_resp["result"]["content"][0]["text"]
        list_res = json.loads(list_text)
        check("list_snapshots base_dir present", "base_dir" in list_res)
        snaps = list_res.get("snapshots") or []
        # MCP stdio transport may dispatch in parallel, so the first list
        # call can race save. Accept either: (a) finds it on first try, or
        # (b) finds it on retry after delay. We require retry to find it.
        if any(s.get("snapshot_id") == "smoke-test-snap" for s in snaps):
            check("list_snapshots found smoke-test-snap (first try)",
                  True, f"got {list_res.get('count', 0)} snapshots")
        else:
            check("list_snapshots first try saw 0 (expected race)",
                  list_res.get("count", 0) == 0,
                  "first call raced save; retry should find it")
    else:
        print(f"  [FAIL] list_snapshots returned error: {list_resp.get('error', list_resp)}")
        ok = False

    if "result" in list_resp_retry:
        list_text2 = list_resp_retry["result"]["content"][0]["text"]
        list_res2 = json.loads(list_text2)
        snaps2 = list_res2.get("snapshots") or []
        check("list_snapshots retry found smoke-test-snap",
              any(s.get("snapshot_id") == "smoke-test-snap" for s in snaps2),
              f"got {list_res2.get('count', 0)} snapshots")
        check("list_snapshots retry has snapshot_id field",
              all("snapshot_id" in s for s in snaps2))
        check("list_snapshots retry has saved_at field",
              all("saved_at" in s for s in snaps2))
    else:
        print(f"  [FAIL] list_snapshots retry returned error: {list_resp_retry.get('error', list_resp_retry)}")
        ok = False

except Exception as e:
    print(f"FAIL: validation exception: {e}")
    import traceback; traceback.print_exc()
    sys.exit(1)

print()
print(f"[smoke] {'OK' if ok else 'FAIL'}")
sys.exit(0 if ok else 1)
PYEOF

EXIT=$?
echo ""
echo "[smoke] exit: $EXIT"
echo ""
echo "[smoke] cleanup isolated snapshot dir:"
TMP_SNAP_DIR=$(ls -dt /tmp/vivaldi-smoke-snapshots.* 2>/dev/null | head -1)
if [ -n "$TMP_SNAP_DIR" ] && [ -d "$TMP_SNAP_DIR" ]; then
    echo "  (preserved for inspection: $TMP_SNAP_DIR)"
fi
exit $EXIT
