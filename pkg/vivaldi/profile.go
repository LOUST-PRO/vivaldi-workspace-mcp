package vivaldi

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Workspace represents a Vivaldi workspace configuration.
type Workspace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon,omitempty"`
}

// Profile represents a Vivaldi profile containing workspaces and settings.
type Profile struct {
	Path       string
	Workspaces []Workspace
}

// DefaultProfilePath returns the default path to Vivaldi's Default profile directory on Linux.
func DefaultProfilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "vivaldi", "Default")
}

// LoadProfile loads and parses the Vivaldi profile configuration from Preferences.
func LoadProfile(profilePath string) (*Profile, error) {
	if profilePath == "" {
		profilePath = DefaultProfilePath()
	}

	prefFile := filepath.Join(profilePath, "Preferences")
	data, err := os.ReadFile(prefFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read Preferences file: %w", err)
	}

	var root map[string]interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Preferences JSON: %w", err)
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
