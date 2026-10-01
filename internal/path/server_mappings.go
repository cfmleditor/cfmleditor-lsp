package path

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

// Server mappings are application defaults. Application declarations and
// explicit editor configuration override them; no environment values are read.
func loadServerMappings(appDir string) map[string]string {
	out := map[string]string{}

	for dir, n := filepath.Clean(appDir), 0; n < 32; n++ {
		selected := false

		config := filepath.Join(dir, ".cfconfig.json")
		if data, err := DefaultFS.ReadFile(filepath.Join(dir, "server.json")); err == nil {
			var server struct {
				CFConfig struct {
					File string `json:"file"`
				} `json:"cfconfig"`
			}
			if json.Unmarshal(data, &server) == nil && server.CFConfig.File != "" {
				if !staticConfigPath(server.CFConfig.File) {
					return out
				}

				selected = true

				config = server.CFConfig.File
				if !filepath.IsAbs(config) {
					config = filepath.Join(dir, filepath.FromSlash(config))
				}
			}
		}

		if data, err := DefaultFS.ReadFile(config); err == nil {
			var source struct {
				Mappings map[string]json.RawMessage `json:"mappings"`
			}
			if json.Unmarshal(data, &source) != nil {
				return out
			}

			for key, raw := range source.Mappings {
				physical := ""
				if json.Unmarshal(raw, &physical) != nil {
					var entry struct {
						Physical string `json:"physical"`
						Primary  string `json:"primary"`
					}
					if json.Unmarshal(raw, &entry) != nil || strings.EqualFold(entry.Primary, "archive") {
						continue
					}

					physical = entry.Physical
				}

				key = strings.ToLower(strings.Trim(key, "/"))

				if !staticConfigPath(physical) {
					continue
				}

				if !filepath.IsAbs(physical) {
					physical = filepath.Join(filepath.Dir(config), filepath.FromSlash(physical))
				}

				out[key] = filepath.Clean(physical)
			}

			return out
		}

		if selected {
			return out
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return out
		}

		dir = parent
	}

	return out
}

func staticConfigPath(value string) bool {
	return value != "" && !strings.ContainsAny(value, "#$") && !strings.Contains(value, "://") && !strings.HasPrefix(value, "~")
}
