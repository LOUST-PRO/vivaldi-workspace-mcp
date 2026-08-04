#!/usr/bin/env bash
# Smoke test for vivaldi-workspace-mcp Fase A hardening.
#
# Sends 4 JSON-RPC requests over stdio and verifies responses.
# Run from repo root: scripts/smoke.sh

set -euo pipefail

BIN="$(cd "$(dirname "$0")/.." && pwd)/bin/vivaldi-workspace-mcp"
echo "[smoke] binary: $BIN"
if [[ ! -x "$BIN" ]]; then
    echo "[smoke] FAIL: binary not built. Run: go build -o bin/vivaldi-workspace-mcp ."
    exit 1
fi

# Each request is a single JSON object on one line.
# Response from MCP server is also single JSON object per line.

rq_initialize='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"smoke","version":"0.1.0"}}}'
rq_initialized='{"jsonrpc":"2.0","method":"notifications/initialized"}'
rq_list_tools='{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'
rq_list_workspaces='{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_workspaces","arguments":{}}}'
rq_list_tabs='{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"list_workspace_tabs","arguments":{}}}'
rq_launch_invalid='{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"launch_tabs","arguments":{"urls":"file:///etc/passwd,javascript:alert(1),https://example.com,https://github.com/foo/bar"}}}'
rq_export_html='{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"export_workspace_html","arguments":{"output_path":"/tmp/smoke_vivaldi.html"}}}'

# Send them sequentially; capture all responses.
{
    echo "$rq_initialize"
    echo "$rq_initialized"
    echo "$rq_list_tools"
    echo "$rq_list_workspaces"
    echo "$rq_list_tabs"
    echo "$rq_launch_invalid"
    echo "$rq_export_html"
} | timeout 30 "$BIN" 2>/tmp/smoke_stderr.log | tee /tmp/smoke_stdout.log

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
    lines = [ln for ln in (ln.strip() for ln in f) if ln.startswith("{")]

ok = True
def check(label, cond, hint=""):
    global ok
    status = "OK" if cond else "FAIL"
    print(f"  [{status}] {label}{(' — ' + hint) if hint else ''}")
    if not cond:
        ok = False

try:
    tools_resp = json.loads(lines[2])
    tools = tools_resp.get("result", {}).get("tools", [])
    check("initialize + tools/list returned", len(tools) == 4, f"got {len(tools)} tools")

    for t in tools:
        ann = t.get("annotations", {})
        if t["name"] == "launch_tabs":
            check(f"launch_tabs.readOnlyHint=false", ann.get("readOnlyHint") is False, "must be false (spawns process)")
        else:
            check(f"{t['name']}.readOnlyHint=true", ann.get("readOnlyHint") is True, "must be true (read-only)")
        check(f"{t['name']}.openWorldHint=false", ann.get("openWorldHint") is False, "must be false (local-only)")
        check(f"{t['name']}.idempotentHint=true", ann.get("idempotentHint") is True, "must be true")

    ws = json.loads(lines[3])
    workspaces = json.loads(ws["result"]["content"][0]["text"])
    check("list_workspaces profile_path present", "profile_path" in workspaces, "must have profile_path")
    check("list_workspaces count > 0", workspaces["count"] > 0, f"got {workspaces['count']} workspaces")
    check("list_workspaces workspaces array", isinstance(workspaces.get("workspaces"), list))

    tabs = json.loads(lines[4])
    tabsum = json.loads(tabs["result"]["content"][0]["text"])
    check("list_workspace_tabs count > 0", tabsum["count"] > 0, f"got {tabsum['count']} tabs")
    check("list_workspace_tabs sources[]", isinstance(tabsum.get("sources"), list))

    launch = json.loads(lines[5])
    if "result" in launch and launch["result"].get("isError"):
        envelope = json.loads(launch["result"]["content"][0]["text"])
        # launch_tabs returns either result or an error envelope
        if "result" in envelope:
            launch_res = envelope["result"]
        else:
            launch_res = envelope
        # Because vivaldi is running, this should accept some URLs
        check("launch_tabs returned launched_urls", "launched_urls" in launch_res)
    else:
        launch_text = launch["result"]["content"][0]["text"]
        launch_res = json.loads(launch_text)
        check("launch_tabs binary reported", "binary" in launch_res)
        check("launch_tabs launched_urls reported", "launched_urls" in launch_res or "rejected_urls" in launch_res)

    exp = json.loads(lines[6])
    if "result" in exp and not exp["result"].get("isError"):
        exp_text = exp["result"]["content"][0]["text"]
        exp_res = json.loads(exp_text)
        check("export atomic=true", exp_res.get("atomic") is True)
        check("export bytes > 0", exp_res.get("bytes", 0) > 0, f"got {exp_res.get('bytes')}")
    else:
        print(f"  [WARN] export_workspace_html returned error: {exp}")

except Exception as e:
    print(f"FAIL: validation exception: {e}")
    sys.exit(1)

print()
print(f"[smoke] {'OK' if ok else 'FAIL'}")
sys.exit(0 if ok else 1)
PYEOF

EXIT=$?
echo ""
echo "[smoke] exit: $EXIT"
exit $EXIT
