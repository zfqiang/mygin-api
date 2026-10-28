package database

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mygin-api/pkg/logging"
)

func TestGormLoggerFormatsSlowQueryWithoutParameters(t *testing.T) {
	directory := t.TempDir()
	loggers, err := logging.New(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer loggers.Close()

	dbLogger := newGormLogger(loggers.Database).(*gormLogger)
	query := "SELECT * FROM users WHERE username = ?"
	filteredQuery, params := dbLogger.ParamsFilter(context.Background(), query, "sensitive-user")
	if filteredQuery != query || params != nil {
		t.Fatal("SQL parameters should not be included in database logs")
	}

	dbLogger.Trace(context.Background(), time.Now().Add(-time.Second), func() (string, int64) {
		return filteredQuery, 1
	}, nil)
	if err := loggers.Sync(); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(filepath.Join(directory, "database.log"))
	if err != nil {
		t.Fatal(err)
	}
	output := string(content)
	for _, expected := range []string{"SQL 执行缓慢", "duration", "rows_affected", query} {
		if !strings.Contains(output, expected) {
			t.Errorf("database log does not contain %q", expected)
		}
	}
	if strings.Contains(output, "sensitive-user") {
		t.Fatal("database log contains a SQL parameter value")
	}
}
