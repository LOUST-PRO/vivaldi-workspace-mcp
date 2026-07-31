# 🚀 vivaldi-workspace-mcp

Un servidor MCP (Model Context Protocol) escrito en **Go** para inspeccionar, exportar, organizar y gestionar los **Espacios de Trabajo (Workspaces)** y pestañas de Vivaldi Browser en Linux.

---

## 📌 Características

- 📂 **Inspección de Espacios (Workspaces)**: Detecta automáticamente todos los espacios (`Developer`, `Investigacion`, `UI`, `WorkHunting`, `Ocio`, `Grupos`) configurados en tu perfil de Vivaldi.
- 🔖 **Extracción y Rescate de Sesiones**: Lee de los archivos de sesión binarios de Chromium/Vivaldi (`Tabs_*`) e identifica todas las URLs abiertas e históricas.
- 📊 **Exportación HTML Interactiva**: Genera páginas de rescate organizadas por dominios y con filtro de búsqueda en tiempo real.
- 🚀 **Lanzador de Pestañas**: Abre grupos de pestañas o enlaces directamente en la instancia activa de Vivaldi.

---

## 🛠️ Herramientas MCP (Tools)

1. `list_workspaces`: Retorna los espacios de trabajo registrados en Vivaldi.
2. `list_workspace_tabs`: Lista las URLs y dominios extraídos de la sesión de Vivaldi.
3. `export_workspace_html`: Genera un reporte HTML filtrable con todas las pestañas recuperadas.
4. `launch_tabs`: Inicia Vivaldi con las URLs pasadas por parámetro.

---

## 🚀 Compilación y Ejecución

```bash
# Compilar el binario
go build -o bin/vivaldi-workspace-mcp .

# Configuración en tu cliente MCP (ej. ~/.gemini/mcp_config.json o Claude Code / Antigravity)
{
  "mcpServers": {
    "vivaldi-workspace": {
      "command": "/home/lou/Proyectos/OSS/vivaldi-workspace-mcp/bin/vivaldi-workspace-mcp"
    }
  }
}
```

---

## 📜 Licencia

MIT © [louzt / LOUST-PRO](https://github.com/LOUST-PRO)
