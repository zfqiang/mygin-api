package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewSeparatesLogCategories(t *testing.T) {
	directory := t.TempDir()
	loggers, err := New(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer loggers.Close()

	loggers.System.Info("system marker")
	loggers.Database.Info("database marker")
	loggers.Redis.Info("redis marker")
	if err := loggers.Sync(); err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"system.log":   "system marker",
		"database.log": "database marker",
		"redis.log":    "redis marker",
	}
	contents := make(map[string]string, len(want))
	for filename, marker := range want {
		content, err := os.ReadFile(filepath.Join(directory, filename))
		if err != nil {
			t.Fatal(err)
		}
		contents[filename] = string(content)
		if !strings.Contains(string(content), marker) {
			t.Errorf("%s does not contain its category marker", filename)
		}
	}
	for filename, content := range contents {
		for otherFile, marker := range want {
			if filename != otherFile && strings.Contains(content, marker) {
				t.Errorf("%s contains the %s marker", filename, otherFile)
			}
		}
	}
}
