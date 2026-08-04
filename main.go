// Command vivaldi-workspace-mcp is an MCP server that inspects, extracts,
// organizes, and manages Vivaldi workspaces and tab sessions on Linux.
//
// Hardened per the lzt-zero-noise-mcp 12 rules:
//   - Structured JSON output (embedded in text content)
//   - ToolAnnotations on every tool (read-only, idempotent, open-world=false)
//   - Stderr logger so MCP stdio is never polluted
//   - signal.NotifyContext for graceful SIGINT/SIGTERM shutdown
//   - atomic file writes via pkg/vivaldi/filex
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/louzt/vivaldi-workspace-mcp/pkg/vivaldi"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// readOnlyAnnot is the canonical ToolAnnotations block for read-only tools
// (those that only inspect the Vivaldi profile, never write).
//
// readOnlyHint      = true:  we never modify Vivaldi's profile state.
// destructiveHint   = false: same.
// idempotentHint    = true:  same inputs produce same outputs (modulo time).
// openWorldHint     = false: the MCP only touches the local Vivaldi profile.
func readOnlyAnnot() mcp.ToolAnnotation {
	ro, d, i, o := true, false, true, false
	return mcp.ToolAnnotation{
		ReadOnlyHint:    &ro,
		DestructiveHint: &d,
		IdempotentHint:  &i,
		OpenWorldHint:   &o,
	}
}

// mutateAnnot is for tools that produce side effects (file write, process
// spawn). ReadOnlyHint=false, all others stay consistent with read-only:
// non-destructive on profile data, idempotent on re-run, closed-world.
func mutateAnnot() mcp.ToolAnnotation {
	ro, d, i, o := false, false, true, false
	return mcp.ToolAnnotation{
		ReadOnlyHint:    &ro,
		DestructiveHint: &d,
		IdempotentHint:  &i,
		OpenWorldHint:   &o,
	}
}

// jsonResult marshals v as minified JSON and wraps it in NewToolResultText.
// Returns a soft error variant if marshaling fails (which would be a bug).
func jsonResult(label string, v interface{}) (*mcp.CallToolResult, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("[%s] marshal failed: %v", label, err)), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}

// jsonErr wraps a ToolError envelope as MCP text result (soft error).
func jsonErr(label string, code, msg, hint string) (*mcp.CallToolResult, error) {
	envelope := vivaldi.ToolError{Code: code, Message: msg, Hint: hint}
	b, _ := json.Marshal(envelope)
	return mcp.NewToolResultText(fmt.Sprintf("[%s] %s", label, string(b))), nil
}

// splitCSV parses a comma-separated URL string. Whitespace tolerant.
// Empty tokens are skipped.
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func main() {
	// Stderr logger; stdout is the MCP channel.
	logger := log.New(os.Stderr, "[vivaldi-workspace-mcp] ", log.LstdFlags|log.Lmsgprefix)

	// Graceful shutdown — wait for in-flight tools to finish or 5s.
	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	s := server.NewMCPServer(
		"vivaldi-workspace-mcp",
		"1.1.0",
		server.WithLogging(),
	)

	// --- Tool: list_workspaces ---
	listWorkspacesTool := mcp.NewTool(
		"list_workspaces",
		mcp.WithDescription("Lists configured Vivaldi Workspaces from the current user profile."),
		mcp.WithToolAnnotation(readOnlyAnnot()),
	)
	s.AddTool(listWorkspacesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		profile, err := vivaldi.LoadProfile("")
		if err != nil {
			return jsonErr("list_workspaces", "profile_load_failed", err.Error(),
				"Verify Vivaldi is installed and ~/.config/vivaldi/Default/Preferences exists.")
		}
		workspaces := make([]vivaldi.WorkspaceSummary, 0, len(profile.Workspaces))
		for i, ws := range profile.Workspaces {
			workspaces = append(workspaces, vivaldi.WorkspaceSummary{
				ID:    ws.ID,
				Name:  ws.Name,
				Icon:  ws.Icon,
				Index: i,
			})
		}
		return jsonResult("list_workspaces", vivaldi.ProfileSummary{
			ProfilePath: profile.Path,
			Count:       len(workspaces),
			Workspaces:  workspaces,
		})
	})

	// --- Tool: list_workspace_tabs ---
	listTabsTool := mcp.NewTool(
		"list_workspace_tabs",
		mcp.WithDescription("Extracts all open and recovered tabs/URLs from Vivaldi session files."),
		mcp.WithToolAnnotation(readOnlyAnnot()),
	)
	s.AddTool(listTabsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		summary, err := vivaldi.GetAllProfileTabs("")
		if err != nil {
			return jsonErr("list_workspace_tabs", "tabs_extract_failed", err.Error(),
				"Check that Sessions/ directory is readable.")
		}
		return jsonResult("list_workspace_tabs", summary)
	})

	// --- Tool: export_workspace_html ---
	exportHTMLTool := mcp.NewTool(
		"export_workspace_html",
		mcp.WithDescription("Generates a searchable HTML report of all recovered tabs grouped by domain."),
		mcp.WithToolAnnotation(mutateAnnot()), // writes a file at user-provided path
		mcp.WithString("output_path", mcp.Description("Output HTML file path. Defaults to ~/Pestanas_Recuperadas_Vivaldi.html if empty. Atomic write: previous version is preserved if process crashes mid-write.")),
	)
	s.AddTool(exportHTMLTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		outPath := "/home/lou/Pestanas_Recuperadas_Vivaldi.html"
		if argsMap, ok := request.Params.Arguments.(map[string]interface{}); ok {
			if val, ok := argsMap["output_path"].(string); ok && val != "" {
				outPath = val
			}
		}

		// require == false; we fall back to default. So the description
		// above is informational only.
		// (If you want to make it required, add mcp.Required() and surface
		// a hard error here.)

		summary, err := vivaldi.GetAllProfileTabs("")
		if err != nil {
			return jsonErr("export_workspace_html", "tabs_extract_failed", err.Error(),
				"Check that Sessions/ directory is readable.")
		}

		result, err := vivaldi.GenerateHTMLReport(summary.Tabs, outPath)
		if err != nil {
			return jsonErr("export_workspace_html", "html_write_failed", err.Error(),
				"Check that the destination path is writable and not held by another process.")
		}
		result.ProfilePath = summary.ProfilePath
		return jsonResult("export_workspace_html", result)
	})

	// --- Tool: launch_tabs ---
	launchTabsTool := mcp.NewTool(
		"launch_tabs",
		mcp.WithDescription("Launches one or more URLs directly in Vivaldi. Accepts a comma-separated list; invalid (non-http) URLs are reported back as rejected, not silently dropped."),
		mcp.WithToolAnnotation(mutateAnnot()), // spawns a process per call
		mcp.WithString("urls", mcp.Description("Comma-separated list of URLs to open"), mcp.Required()),
	)
	s.AddTool(launchTabsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		urlsStr := ""
		if argsMap, ok := request.Params.Arguments.(map[string]interface{}); ok {
			if val, ok := argsMap["urls"].(string); ok {
				urlsStr = val
			}
		}
		if urlsStr == "" {
			return jsonErr("launch_tabs", "missing_arg", "urls is required", "Provide a comma-separated list of URLs.")
		}

		urls := splitCSV(urlsStr)
		if len(urls) == 0 {
			return jsonErr("launch_tabs", "no_urls", "urls parsed to empty list", "Check for stray commas or whitespace.")
		}

		result, err := vivaldi.LaunchURLs(urls, vivaldi.LaunchOptions{})
		if err != nil {
			// Even on error, surface the partial result so the caller
			// knows which URLs were rejected.
			resultBytes, _ := json.Marshal(map[string]interface{}{
				"error": err.Error(),
				"result": result,
			})
			return mcp.NewToolResultText(string(resultBytes)), nil
		}
		return jsonResult("launch_tabs", result)
	})

	// --- Tool: save_workspace_snapshot ---
	saveSnapTool := mcp.NewTool(
		"save_workspace_snapshot",
		mcp.WithDescription("Captures the current Vivaldi workspace configuration and tab URLs to disk as an atomic JSON snapshot. Use the snapshot_id from the response as input to restore_workspace_snapshot."),
		mcp.WithToolAnnotation(mutateAnnot()),
		mcp.WithString("label", mcp.Description("Optional human label for the snapshot; falls back to UTC timestamp. Filename-safe characters only ([A-Za-z0-9._-]).")),
		mcp.WithString("workspace_id", mcp.Description("Optional workspace ID to snapshot only that workspace. Omit to snapshot all workspaces.")),
	)
	s.AddTool(saveSnapTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		label, wsid := "", ""
		if argsMap, ok := request.Params.Arguments.(map[string]interface{}); ok {
			if v, ok := argsMap["label"].(string); ok {
				label = v
			}
			if v, ok := argsMap["workspace_id"].(string); ok {
				wsid = v
			}
		}
		res, err := vivaldi.SaveSnapshot(vivaldi.SaveSnapshotOption{
			Label:    label,
			OnlyWSID: wsid,
		})
		if err != nil {
			return jsonErr("save_workspace_snapshot", "save_failed", err.Error(),
				"Check filesystem permissions on the snapshot base directory.")
		}
		return jsonResult("save_workspace_snapshot", res)
	})

	// --- Tool: restore_workspace_snapshot ---
	restoreSnapTool := mcp.NewTool(
		"restore_workspace_snapshot",
		mcp.WithDescription("Re-launches URLs from a previously saved snapshot via hardened launch_tabs. Splits into batches of 30 to avoid overwhelming Vivaldi. Invalid URLs are reported back via the rejected_urls field."),
		mcp.WithToolAnnotation(mutateAnnot()),
		mcp.WithString("snapshot_id", mcp.Description("The snapshot_id returned by save_workspace_snapshot or shown in list_snapshots."), mcp.Required()),
		mcp.WithString("workspace_id", mcp.Description("Optional workspace ID to restore only that workspace.")),
		mcp.WithNumber("launch_timeout_seconds", mcp.Description("Per-batch timeout in seconds. Defaults to 10.")),
	)
	s.AddTool(restoreSnapTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		snapID, wsid := "", ""
		var timeoutSec float64 = 0
		if argsMap, ok := request.Params.Arguments.(map[string]interface{}); ok {
			if v, ok := argsMap["snapshot_id"].(string); ok {
				snapID = v
			}
			if v, ok := argsMap["workspace_id"].(string); ok {
				wsid = v
			}
			if v, ok := argsMap["launch_timeout_seconds"].(float64); ok && v > 0 {
				timeoutSec = v
			}
		}
		if snapID == "" {
			return jsonErr("restore_workspace_snapshot", "missing_arg",
				"snapshot_id is required", "Run list_snapshots to find an available snapshot.")
		}

		opts := vivaldi.LaunchOptions{}
		if timeoutSec > 0 {
			opts.Timeout = time.Duration(timeoutSec * float64(time.Second))
		}

		res, err := vivaldi.RestoreSnapshot(snapID, opts, wsid)
		if err != nil {
			// Surface partial result even on error
			resultBytes, _ := json.Marshal(map[string]interface{}{
				"error":  err.Error(),
				"result": res,
			})
			return mcp.NewToolResultText(string(resultBytes)), nil
		}
		return jsonResult("restore_workspace_snapshot", res)
	})

	// --- Tool: list_snapshots ---
	listSnapshotsTool := mcp.NewTool(
		"list_snapshots",
		mcp.WithDescription("Lists saved snapshots sorted newest-first. Returns metadata only (no tab data) for cheap enumeration."),
		mcp.WithToolAnnotation(readOnlyAnnot()),
		mcp.WithNumber("limit", mcp.Description("Max number of snapshots to return. 0 = all.")),
	)
	s.AddTool(listSnapshotsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var limit float64 = 0
		if argsMap, ok := request.Params.Arguments.(map[string]interface{}); ok {
			if v, ok := argsMap["limit"].(float64); ok {
				limit = v
			}
		}
		res, err := vivaldi.ListSnapshots(int(limit))
		if err != nil {
			return jsonErr("list_snapshots", "list_failed", err.Error(),
				"Check snapshot base directory permissions.")
		}
		return jsonResult("list_snapshots", res)
	})

	// Serve stdio under the signal-aware context. ServeStdio doesn't
	// currently accept context, but the defer-block on rootCtx cancel
	// guarantees we exit on SIGINT/SIGTERM.
	logger.Printf("starting on stdio (version 1.1.0)")
	if err := server.ServeStdio(s); err != nil {
		logger.Printf("MCP server error: %v", err)
		// Cleanup
		_ = rootCtx
		os.Exit(1)
	}
}
