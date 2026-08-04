# Documentation

End-user and reviewer-facing documentation for `vivaldi-workspace-mcp`.

If you are new to the project, read in this order:

1. **[mcp-protocol.md](mcp-protocol.md)** — what MCP is, the JSON-RPC
   framing, and how a tool call reaches this server. Required reading if
   you have never used MCP before.
2. **[architecture.md](architecture.md)** — how this server fits into
   your Linux desktop, how it reads Vivaldi's profile, and the data
   flow for each tool.
3. **[security-model.md](security-model.md)** — what this server can
   and cannot do on your system, and why every tool carries explicit
   `ToolAnnotations`.
4. **[snapshots.md](snapshots.md)** — file format, schema stability
   contract, and how to roll back if a snapshot becomes unusable.
5. **[go-concurrency.md](go-concurrency.md)** — Go-specific notes on
   the stdio transport, the `mark3labs/mcp-go` SDK quirks we work
   around, and the concurrency model for tool handlers.

These documents are written for two audiences:

- **End users** who want to understand what a tool does before calling
  it, or who want to debug "why didn't this work" without reading the
  Go source.
- **Reviewers** who want to verify the server is safe to install on
  their system, or who want to evaluate the project as a reference
  implementation of a local-only MCP server.

The Go source has its own package-level doc comments (`pkg/vivaldi/*.go`,
`pkg/vivaldi/filex/*.go`, `main.go`) that go deeper on internals.
