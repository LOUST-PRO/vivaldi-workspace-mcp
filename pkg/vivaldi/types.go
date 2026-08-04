// Package vivaldi provides response types and helpers for the MCP server.
//
// Output structs in this file are designed for structured JSON
// serialization per the lzt-zero-noise-mcp rule:
//   - Stable field names (no abbreviations)
//   - Omit empty slices (not nil)
//   - Time and count fields always present, even if zero
package vivaldi

// WorkspaceSummary describes a single Vivaldi workspace entry.
type WorkspaceSummary struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Icon  string `json:"icon,omitempty"`
	Index int    `json:"index"`
}

// ProfileSummary is the response of list_workspaces.
type ProfileSummary struct {
	ProfilePath string             `json:"profile_path"`
	Count       int                `json:"count"`
	Workspaces  []WorkspaceSummary `json:"workspaces"`
}

// TabSummary describes a single tab extracted from Vivaldi session files.
type TabSummary struct {
	URL    string `json:"url"`
	Domain string `json:"domain"`
}

// TabsSummary is the response of list_workspace_tabs.
//
// Sources is the list of session file basenames that contributed tabs.
// Useful for diagnosing "where did this URL come from" without an extra call.
type TabsSummary struct {
	ProfilePath    string       `json:"profile_path"`
	Count          int          `json:"count"`
	SessionsDir    string       `json:"sessions_dir"`
	Sources        []string     `json:"sources"`
	Tabs           []TabSummary `json:"tabs"`
	Truncated      bool         `json:"truncated,omitempty"`
	TruncatedAfter int          `json:"truncated_after,omitempty"`
}

// HTMLExportResult is the response of export_workspace_html.
type HTMLExportResult struct {
	ProfilePath string `json:"profile_path"`
	OutputPath  string `json:"output_path"`
	Count       int    `json:"count"`
	Bytes       int64  `json:"bytes"`
	Atomic      bool   `json:"atomic"`
	DurationMS  int64  `json:"duration_ms"`
}

// LaunchResult is the response of launch_tabs.
//
// Vivaldi is started ONCE per call with all valid URLs as positional args.
// Invalid URLs are reported back, not silently dropped.
type LaunchResult struct {
	Binary       string   `json:"binary"`
	PID          int      `json:"pid"`
	RequestedURLs []string `json:"requested_urls"`
	LaunchedURLs []string `json:"launched_urls"`
	RejectedURLs []string `json:"rejected_urls"`
	DurationMS   int64    `json:"duration_ms"`
}

// ToolError is the canonical error envelope. Returned via
// mcp.NewToolResultText with embedded JSON when a soft error occurs
// (input validation, recoverable file-not-found, etc).
// Hard errors (panic, permission denied on tool internals) still use
// mcp.NewToolResultError.
type ToolError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

// Rejection describes a single URL rejected by the launcher.
type Rejection struct {
	URL   string `json:"url"`
	Code  string `json:"code"`
	Why   string `json:"why"`
}

// LaunchValidation is the input-side validation report returned
// before any process is started. Used by main.go for context-with-code.
type LaunchValidation struct {
	Accepted   []string    `json:"accepted"`
	Rejected   []Rejection `json:"rejected"`
}
