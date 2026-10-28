package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// LoadEnvFromRoot finds the project root and returns its .env path.
func LoadEnvFromRoot() (string, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("get config source path")
	}

	dir := filepath.Dir(filename)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, ".env"), nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("find project root: go.mod not found")
		}
		dir = parent
	}
}
