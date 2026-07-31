package vivaldi

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Workspace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon,omitempty"`
}

type Profile struct {
	Path       string
	Workspaces []Workspace
}

func DefaultProfilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "vivaldi", "Default")
}

func LoadProfile(profilePath string) (*Profile, error) {
	if profilePath == "" {
		profilePath = DefaultProfilePath()
	}

	prefFile := filepath.Join(profilePath, "Preferences")
	data, err := os.ReadFile(prefFile)
	if err != nil {
		return nil, fmt.Errorf("error leyendo Preferences: %w", err)
	}

	var root map[string]interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("error deserializando Preferences JSON: %w", err)
	}

	p := &Profile{
		Path:       profilePath,
		Workspaces: make([]Workspace, 0),
	}

	if vivaldiData, ok := root["vivaldi"].(map[string]interface{}); ok {
		if wsData, ok := vivaldiData["workspaces"].(map[string]interface{}); ok {
			if wsList, ok := wsData["list"].([]interface{}); ok {
				for _, item := range wsList {
					if wsMap, ok := item.(map[string]interface{}); ok {
						idStr := fmt.Sprintf("%v", wsMap["id"])
						nameStr := fmt.Sprintf("%v", wsMap["name"])
						iconStr := ""
						if ic, ok := wsMap["icon"].(string); ok {
							iconStr = ic
						}

						p.Workspaces = append(p.Workspaces, Workspace{
							ID:   idStr,
							Name: nameStr,
							Icon: iconStr,
						})
					}
				}
			}
		}
	}

	return p, nil
}
