# 🚀 vivaldi-workspace-mcp (Español)

Servidor MCP (Model Context Protocol) escrito en **Go** para inspeccionar, extraer, organizar y gestionar **Espacios de Trabajo (Workspaces)** y pestañas del navegador Vivaldi en Linux.

*Lee este documento en [English](README.md).*

---

## 📌 Características

- 📂 **Detección de Espacios**: Lee la configuración de Vivaldi (`Preferences`) para listar Espacios de Trabajo activos.
- 🔖 **Extracción de Pestañas**: Escanea archivos binarios de sesión (`Tabs_*`) para recuperar URLs y páginas abiertas.
- 📊 **Reporte HTML Interactivo**: Exporta reportes HTML organizados por dominio con filtro de búsqueda en tiempo real.
- 🚀 **Lanzador de Pestañas**: Abre grupos de enlaces directamente en la instancia de Vivaldi.

---

## 🛠️ Herramientas (Tools) MCP

| Herramienta | Descripción |
| :--- | :--- |
| `list_workspaces` | Lista los Espacios configurados en el perfil de Vivaldi. |
| `list_workspace_tabs` | Retorna las URLs y pestañas extraídas de los archivos de sesión. |
| `export_workspace_html` | Genera un reporte HTML filtrable con todas las pestañas recuperadas. |
| `launch_tabs` | Inicia Vivaldi con las URLs proporcionadas. |

---

## 🚀 Compilación y Configuración

```bash
# Compilar el binario
go build -o bin/vivaldi-workspace-mcp .
```

En tu cliente MCP (`mcp_servers`):

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

## 📄 Licencia

Licencia MIT.
