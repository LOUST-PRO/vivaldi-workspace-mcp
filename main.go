package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/louzt/vivaldi-workspace-mcp/pkg/vivaldi"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	s := server.NewMCPServer(
		"vivaldi-workspace-mcp",
		"1.0.0",
		server.WithLogging(),
	)

	// Tool: list_workspaces
	listWorkspacesTool := mcp.NewTool(
		"list_workspaces",
		mcp.WithDescription("Lists configured Vivaldi Workspaces from the current user profile."),
	)
	s.AddTool(listWorkspacesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		profile, err := vivaldi.LoadProfile("")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to load Vivaldi profile: %v", err)), nil
		}

		res := fmt.Sprintf("Total Workspaces configured: %d\n", len(profile.Workspaces))
		for i, ws := range profile.Workspaces {
			res += fmt.Sprintf("%d. %s (ID: %s)\n", i+1, ws.Name, ws.ID)
		}

		return mcp.NewToolResultText(res), nil
	})

	// Tool: list_workspace_tabs
	listTabsTool := mcp.NewTool(
		"list_workspace_tabs",
		mcp.WithDescription("Extracts all open and recovered tabs/URLs from Vivaldi session files."),
	)
	s.AddTool(listTabsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		tabs, err := vivaldi.GetAllProfileTabs("")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to extract tabs: %v", err)), nil
		}

		res := fmt.Sprintf("Total Tabs/URLs recovered: %d\n\n", len(tabs))
		maxDisplay := 50
		if len(tabs) < maxDisplay {
			maxDisplay = len(tabs)
		}
		for i := 0; i < maxDisplay; i++ {
			res += fmt.Sprintf("- [%s] %s\n", tabs[i].Domain, tabs[i].URL)
		}
		if len(tabs) > maxDisplay {
			res += fmt.Sprintf("\n... and %d more tabs.", len(tabs)-maxDisplay)
		}

		return mcp.NewToolResultText(res), nil
	})

	// Tool: export_workspace_html
	exportHTMLTool := mcp.NewTool(
		"export_workspace_html",
		mcp.WithDescription("Generates an interactive searchable HTML report of all recovered tabs grouped by domain."),
		mcp.WithString("output_path", mcp.Description("Output HTML file path"), mcp.Required()),
	)
	s.AddTool(exportHTMLTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		outPath := "/home/lou/Pestanas_Recuperadas_Vivaldi.html"
		if argsMap, ok := request.Params.Arguments.(map[string]interface{}); ok {
			if val, ok := argsMap["output_path"].(string); ok && val != "" {
				outPath = val
			}
		}

		tabs, err := vivaldi.GetAllProfileTabs("")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to read tabs: %v", err)), nil
		}

		if err := vivaldi.GenerateHTMLReport(tabs, outPath); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to generate HTML report: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("HTML report successfully generated at: %s (%d tabs).", outPath, len(tabs))), nil
	})

	// Tool: launch_tabs
	launchTabsTool := mcp.NewTool(
		"launch_tabs",
		mcp.WithDescription("Launches one or more URLs directly in Vivaldi."),
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
			return mcp.NewToolResultError("At least one URL is required"), nil
		}

		urls := []string{urlsStr}
		if err := vivaldi.LaunchURLsInVivaldi(urls); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to launch Vivaldi: %v", err)), nil
		}

		return mcp.NewToolResultText("Vivaldi launched with provided URLs."), nil
	})

	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("MCP server error: %v\n", err)
		os.Exit(1)
	}
}
