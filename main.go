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
		mcp.WithDescription("Lista los Espacios de Trabajo (Workspaces) configurados en el perfil de Vivaldi."),
	)
	s.AddTool(listWorkspacesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		profile, err := vivaldi.LoadProfile("")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error cargando perfil: %v", err)), nil
		}

		res := fmt.Sprintf("Total de Espacios configurados: %d\n", len(profile.Workspaces))
		for i, ws := range profile.Workspaces {
			res += fmt.Sprintf("%d. %s (ID: %s)\n", i+1, ws.Name, ws.ID)
		}

		return mcp.NewToolResultText(res), nil
	})

	// Tool: list_workspace_tabs
	listTabsTool := mcp.NewTool(
		"list_workspace_tabs",
		mcp.WithDescription("Extrae todas las pestañas y URLs rescatadas de las sesiones de Vivaldi."),
	)
	s.AddTool(listTabsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		tabs, err := vivaldi.GetAllProfileTabs("")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error extrayendo pestañas: %v", err)), nil
		}

		res := fmt.Sprintf("Total de Pestañas/URLs recuperadas: %d\n\n", len(tabs))
		maxDisplay := 50
		if len(tabs) < maxDisplay {
			maxDisplay = len(tabs)
		}
		for i := 0; i < maxDisplay; i++ {
			res += fmt.Sprintf("- [%s] %s\n", tabs[i].Domain, tabs[i].URL)
		}
		if len(tabs) > maxDisplay {
			res += fmt.Sprintf("\n... y %d pestañas más.", len(tabs)-maxDisplay)
		}

		return mcp.NewToolResultText(res), nil
	})

	// Tool: export_workspace_html
	exportHTMLTool := mcp.NewTool(
		"export_workspace_html",
		mcp.WithDescription("Genera un reporte interactivo en HTML con todas las pestañas recuperadas organizadas por dominio."),
		mcp.WithString("output_path", mcp.Description("Ruta del archivo HTML de salida"), mcp.Required()),
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
			return mcp.NewToolResultError(fmt.Sprintf("Error leyendo pestañas: %v", err)), nil
		}

		if err := vivaldi.GenerateHTMLReport(tabs, outPath); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error generando reporte HTML: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Reporte HTML generado exitosamente en: %s con %d pestañas.", outPath, len(tabs))), nil
	})

	// Tool: launch_tabs
	launchTabsTool := mcp.NewTool(
		"launch_tabs",
		mcp.WithDescription("Abre una o varias URLs directamente en Vivaldi."),
		mcp.WithString("urls", mcp.Description("Lista de URLs separadas por coma"), mcp.Required()),
	)
	s.AddTool(launchTabsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		urlsStr := ""
		if argsMap, ok := request.Params.Arguments.(map[string]interface{}); ok {
			if val, ok := argsMap["urls"].(string); ok {
				urlsStr = val
			}
		}

		if urlsStr == "" {
			return mcp.NewToolResultError("Se requiere al menos una URL"), nil
		}

		urls := []string{urlsStr}
		if err := vivaldi.LaunchURLsInVivaldi(urls); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error abriendo Vivaldi: %v", err)), nil
		}

		return mcp.NewToolResultText("Iniciado Vivaldi con las URLs proporcionadas."), nil
	})

	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("Error ejecutando MCP server: %v\n", err)
		os.Exit(1)
	}
}
