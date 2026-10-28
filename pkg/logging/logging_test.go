package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var _ interface {
	LogF(string, ...any)
	WarnF(string, ...any)
	Errorf(string, ...any)
	Fatal(...any)
} = (*Logger)(nil)

func TestNewSeparatesLogCategories(t *testing.T) {
	directory := t.TempDir()
	loggers, err := New(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer loggers.Close()

	loggers.System.LogF("system marker | request_id=%s", "req-123")
	loggers.System.WarnF("warning marker")
	loggers.System.Errorf("%s", "error marker")
	loggers.System.Errorf("formatted error: %s", "failure")
	loggers.System.LogF("formatted marker: %s", "LogF")
	loggers.Database.LogF("database marker | query=%s", "SELECT 1")
	loggers.Redis.LogF("redis marker | command=%s", "PING")
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
		if !strings.Contains(string(content), time.Now().Format("2006-01-02T")) {
			t.Errorf("%s does not contain an ISO-formatted timestamp", filename)
		}
		if !strings.Contains(string(content), "INFO") {
			t.Errorf("%s does not contain a formatted log level", filename)
		}
	}
	systemContent := contents["system.log"]
	for _, marker := range []string{"WARN", "ERROR", "formatted error: failure", "formatted marker: LogF"} {
		if !strings.Contains(systemContent, marker) {
			t.Errorf("system log does not contain %q", marker)
		}
	}
	if !strings.Contains(systemContent, "logging_test.go") {
		t.Fatal("logger wrapper did not preserve the original caller location")
	}
	if !strings.Contains(contents["system.log"], "request_id") ||
		!strings.Contains(contents["database.log"], "query") ||
		!strings.Contains(contents["redis.log"], "command") {
		t.Fatal("structured fields were not included in formatted logs")
	}
	for _, expected := range []string{
		"INFO | system |",
		"system marker | request_id=req-123",
		"formatted error: failure",
		"formatted marker: LogF",
	} {
		if !strings.Contains(contents["system.log"], expected) {
			t.Errorf("system log does not contain formatted output %q", expected)
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

func TestFormatMessageRendersReadableFields(t *testing.T) {
	got := formatMessage([]any{
		"request finished",
		"method", "GET",
		"path", "/api/v1/orders",
		"error", "connection timed out",
		"attempt",
	})
	want := `request finished | method=GET | path=/api/v1/orders | error="connection timed out" | attempt=<missing>`
	if got != want {
		t.Fatalf("formatMessage() = %q, want %q", got, want)
	}
}
