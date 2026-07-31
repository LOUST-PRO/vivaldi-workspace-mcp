# Plan de Arquitectura: `vivaldi-workspace-mcp` (Go / MCP Protocol)

## 📌 Objetivo
Desarrollar un Servidor MCP (Model Context Protocol) en **Go** (usando `mark3labs/mcp-go`) para inspeccionar, guardar, organizar y restaurar pestañas y **Espacios de Trabajo (Workspaces)** del navegador Vivaldi en Linux.

---

## 🏗️ Estructura del Proyecto
```
vivaldi-workspace-mcp/
├── .gitignore
├── go.mod
├── go.sum
├── main.go
├── plan.md
├── README.md
└── pkg/
    └── vivaldi/
        ├── profile.go      # Lectura de Preferences (Workspaces list & settings)
        ├── session.go      # Parseo de Sessions (Tabs y URLs)
        └── launcher.go     # Restauración y apertura de pestañas en Vivaldi via CLI
```

---

## 🛠️ Herramientas (Tools) MCP expuestas

1. **`list_workspaces`**
   - Retorna la lista de Espacios de Vivaldi configurados (`Developer`, `Investigacion`, `Ocio`, `Grupos`, `UI`, `WorkHunting`) y su metadata.

2. **`list_workspace_tabs`**
   - Retorna todas las pestañas (URLs) recuperadas de las sesiones del perfil.

3. **`export_workspace_html`**
   - Genera una página HTML interactiva y organizada por dominios con todas las pestañas recuperadas.

4. **`launch_tabs`**
   - Abre un grupo de pestañas o un espacio en Vivaldi mediante la interfaz CLI (`vivaldi <urls>`).

5. **`save_workspace_snapshot`**
   - Crea un respaldo JSON de la sesión actual por espacio para recuperación instantánea.

---

## 🧪 Estrategia de Validación
- `go test ./...` para pruebas unitarias de parseo de JSON e inspección de sesión.
- Verificación en tiempo real contra `~/.config/vivaldi/Default/Preferences`.
