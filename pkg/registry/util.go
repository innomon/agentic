package registry

import (
	"os"
	"strings"
)

// ExpandEnvWithDefaults expands ${VAR:-default} and ${VAR} patterns in s.
func ExpandEnvWithDefaults(s string) string {
	return os.Expand(s, func(key string) string {
		if name, def, ok := strings.Cut(key, ":-"); ok {
			if v := os.Getenv(name); v != "" {
				return v
			}
			return def
		}
		return os.Getenv(key)
	})
}
