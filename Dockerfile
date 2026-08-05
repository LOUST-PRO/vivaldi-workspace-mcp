# syntax=docker/dockerfile:1.7
#
# Multi-stage Dockerfile for vivaldi-workspace-mcp.
#
# Glama.ai's registry builds the image from this Dockerfile (or infers one
# from the repo if absent — see https://glama.ai/mcp/methodology). Keeping
# the build explicit here means Glama's introspection runs against the
# exact same artifact we ship, with no hidden AI-inference step.
#
# Image characteristics:
#   - Static binary (CGO_ENABLED=0, no glibc dependency) on Alpine 3.22.
#   - Runs as UID 65532 (the distroless-style "nonroot" convention).
#   - JSON-RPC frames travel on stdout, diagnostics on stderr. Do not
#     detach, redirect, or pipe — any I/O transformation will break the
#     MCP transport.
#
# Caveats for the Glama introspection environment:
#   - This image does NOT ship a Vivaldi profile. The server starts and
#     responds to `initialize` / `tools/list` without one (the SDK
#     handles those before any tool handler runs). Individual tool calls
#     that touch the profile will return a structured ToolError envelope
#     rather than crashing; this is by design and matches the production
#     behavior on a host without Vivaldi installed.
#   - No network egress. The runtime never opens a socket, so behavioral
#     analysis of the container should report "no outbound traffic".

# ---- Stage 1: build --------------------------------------------------------
FROM golang:1.26.5-alpine3.24 AS builder

WORKDIR /src

# Pull modules first so they cache across source-only edits.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# -trimpath strips the local $GOPATH so the binary is reproducible
# across build hosts. -s -w drops the symbol/debug tables.
RUN CGO_ENABLED=0 GOOS=linux \
    go build \
        -trimpath \
        -ldflags='-s -w' \
        -o /out/vivaldi-workspace-mcp \
        .

# ---- Stage 2: runtime ------------------------------------------------------
FROM alpine:3.24

# UID 65532 matches the "nonroot" convention used by distroless and
# most hardened container base images.
RUN addgroup -g 65532 app \
    && adduser  -u 65532 -G app -D app

WORKDIR /app

COPY --from=builder /out/vivaldi-workspace-mcp /app/vivaldi-workspace-mcp

USER app:app

ENTRYPOINT ["/app/vivaldi-workspace-mcp"]

# ---- Metadata --------------------------------------------------------------
LABEL org.opencontainers.image.title="vivaldi-workspace-mcp" \
      org.opencontainers.image.description="MCP server for inspecting, extracting, and managing Vivaldi browser workspaces and tab sessions on Linux." \
      org.opencontainers.image.source="https://github.com/LOUST-PRO/vivaldi-workspace-mcp" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.vendor="LOUST-PRO"