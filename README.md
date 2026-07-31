# 🚀 vivaldi-workspace-mcp

[![Go Version](https://img.shields.io/badge/go-1.26+-00ADD8.svg?style=flat&logo=go)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

An Model Context Protocol (**MCP**) server written in **Go** designed to inspect, extract, organize, and manage **Vivaldi Workspaces** and tab sessions on Linux.

*Read this document in [Español](README.es.md).*

---

## 📌 Features

- 📂 **Workspace Discovery**: Automatically parses Vivaldi profile preferences (`Preferences`) to detect configured Workspaces.
- 🔖 **Session & Tab Extraction**: Reads binary Chromium/Vivaldi session files (`Tabs_*`) to extract open, stored, and recovered URLs.
- 📊 **Interactive HTML Report**: Exports searchable, domain-grouped HTML reports for quick session recovery.
- 🚀 **Tab & Workspace Launcher**: Programmatically launches URLs and tab groups directly in Vivaldi via CLI.

---

## 🛠️ Available MCP Tools

| Tool | Description |
| :--- | :--- |
| `list_workspaces` | Lists all configured Vivaldi Workspaces with their unique IDs. |
| `list_workspace_tabs` | Extracts and returns open/recovered tabs and URLs from session state files. |
| `export_workspace_html` | Generates a searchable HTML report grouping recovered tabs by domain. |
| `launch_tabs` | Launches Vivaldi with a specified list of URLs. |

---

## 💻 Installation & Usage

### 1. Build the Binary

```bash
# Clone the repository
git clone https://github.com/LOUST-PRO/vivaldi-workspace-mcp.git
cd vivaldi-workspace-mcp

# Build executable
go build -o bin/vivaldi-workspace-mcp .
```

### 2. Configure in your MCP Client (e.g. Antigravity CLI / Claude Code)

Add the server definition to your `mcp_servers` configuration:

```json
{
  "mcpServers": {
    "vivaldi-workspace": {
      "command": "/home/lou/Proyectos/OSS/vivaldi-workspace-mcp/bin/vivaldi-workspace-mcp"
    }
  }
}
```

---

## 📄 License

Distributed under the MIT License. See `LICENSE` for details.
