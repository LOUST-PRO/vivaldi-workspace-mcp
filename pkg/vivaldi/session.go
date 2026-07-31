package vivaldi

import (
	"bytes"
	"fmt"
	"html"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type TabInfo struct {
	URL    string `json:"url"`
	Domain string `json:"domain"`
}

type WorkspaceSnapshot struct {
	WorkspaceName string    `json:"workspace_name"`
	TotalTabs     int       `json:"total_tabs"`
	Tabs          []TabInfo `json:"tabs"`
}

var httpRegex = regexp.MustCompile(`https?://[^\s\x00-\x1f\x7f-\xff"]+`)

func ExtractURLsFromSession(sessionFilePath string) ([]TabInfo, error) {
	data, err := os.ReadFile(sessionFilePath)
	if err != nil {
		return nil, fmt.Errorf("error leyendo archivo de sesión %s: %w", sessionFilePath, err)
	}

	matches := httpRegex.FindAll(data, -1)
	seen := make(map[string]bool)
	var tabs []TabInfo

	for _, match := range matches {
		rawURL := string(match)
		cleanURL := strings.TrimRight(rawURL, ").,;'\"\x00")

		if len(cleanURL) < 12 {
			continue
		}

		if strings.Contains(cleanURL, "chrome-extension") || strings.Contains(cleanURL, "vivaldi") || strings.Contains(cleanURL, "favicon") {
			continue
		}

		if !seen[cleanURL] {
			seen[cleanURL] = true
			parsed, err := url.Parse(cleanURL)
			domain := ""
			if err == nil {
				domain = strings.TrimPrefix(parsed.Host, "www.")
			}
			tabs = append(tabs, TabInfo{
				URL:    cleanURL,
				Domain: domain,
			})
		}
	}

	return tabs, nil
}

func GetAllProfileTabs(profilePath string) ([]TabInfo, error) {
	if profilePath == "" {
		profilePath = DefaultProfilePath()
	}

	sessionsDir := filepath.Join(profilePath, "Sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return nil, fmt.Errorf("error listando directorio Sessions: %w", err)
	}

	seen := make(map[string]bool)
	var allTabs []TabInfo

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "Tabs_") {
			fullPath := filepath.Join(sessionsDir, entry.Name())
			tabs, err := ExtractURLsFromSession(fullPath)
			if err != nil {
				continue
			}
			for _, t := range tabs {
				if !seen[t.URL] {
					seen[t.URL] = true
					allTabs = append(allTabs, t)
				}
			}
		}
	}

	return allTabs, nil
}

func GenerateHTMLReport(tabs []TabInfo, outputPath string) error {
	byDomain := make(map[string][]TabInfo)
	for _, t := range tabs {
		d := t.Domain
		if d == "" {
			d = "otros"
		}
		byDomain[d] = append(byDomain[d], t)
	}

	type domainGroup struct {
		Domain string
		Tabs   []TabInfo
	}

	var groups []domainGroup
	for d, tList := range byDomain {
		groups = append(groups, domainGroup{Domain: d, Tabs: tList})
	}

	sort.Slice(groups, func(i, j int) bool {
		return len(groups[i].Tabs) > len(groups[j].Tabs)
	})

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf(`<!DOCTYPE html>
<html lang="es">
<head>
    <meta charset="UTF-8">
    <title>Vivaldi MCP - Pestañas Recuperadas (%d pestañas)</title>
    <style>
        body { font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #0f172a; color: #f8fafc; margin: 0; padding: 24px; }
        .container { max-width: 1000px; margin: 0 auto; }
        header { margin-bottom: 24px; border-bottom: 1px solid #334155; padding-bottom: 16px; }
        h1 { color: #38bdf8; margin: 0 0 8px 0; font-size: 28px; }
        p { color: #94a3b8; margin: 0; }
        .search-box { width: 100%%; padding: 14px 18px; font-size: 16px; border-radius: 10px; border: 1px solid #334155; background: #1e293b; color: #f8fafc; margin-bottom: 24px; box-sizing: border-box; outline: none; }
        .domain-card { background: #1e293b; border-radius: 12px; padding: 18px; margin-bottom: 16px; border: 1px solid #334155; }
        .domain-title { font-size: 18px; font-weight: 700; color: #818cf8; margin-bottom: 12px; display: flex; justify-content: space-between; align-items: center; }
        .badge { background: #334155; color: #f1f5f9; padding: 4px 10px; border-radius: 9999px; font-size: 12px; font-weight: 600; }
        .url-list { list-style: none; padding: 0; margin: 0; }
        .url-item { padding: 8px 0; border-bottom: 1px solid #334155; word-break: break-all; }
        .url-item a { color: #38bdf8; text-decoration: none; font-size: 14px; font-weight: 500; }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>📂 Vivaldi MCP - Pestañas Recuperadas</h1>
            <p>Total de pestañas rescatadas: <strong>%d</strong></p>
        </header>

        <input type="text" id="searchInput" class="search-box" placeholder="🔍 Buscar pestaña o dominio..." onkeyup="filterURLs()">
        <div id="content">
`, len(tabs), len(tabs)))

	for _, g := range groups {
		buf.WriteString(fmt.Sprintf(`
        <div class="domain-card">
            <div class="domain-title">
                <span>🌐 %s</span>
                <span class="badge">%d pestañas</span>
            </div>
            <ul class="url-list">
`, html.EscapeString(g.Domain), len(g.Tabs)))

		for _, t := range g.Tabs {
			buf.WriteString(fmt.Sprintf(`                <li class="url-item"><a href="%s" target="_blank">%s</a></li>
`, html.EscapeString(t.URL), html.EscapeString(t.URL)))
		}

		buf.WriteString(`            </ul>
        </div>
`)
	}

	buf.WriteString(`
        </div>
    </div>

    <script>
        function filterURLs() {
            let input = document.getElementById('searchInput').value.toLowerCase();
            let cards = document.getElementsByClassName('domain-card');
            for (let card of cards) {
                let items = card.getElementsByClassName('url-item');
                let hasMatch = false;
                for (let item of items) {
                    let text = item.innerText.toLowerCase();
                    if (text.includes(input)) {
                        item.style.display = "";
                        hasMatch = true;
                    } else {
                        item.style.display = "none";
                    }
                }
                card.style.display = hasMatch ? "" : "none";
            }
        }
    </script>
</body>
</html>
`)

	return os.WriteFile(outputPath, buf.Bytes(), 0644)
}
