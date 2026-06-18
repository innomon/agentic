package logger

import (
	"bufio"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoggerAndRotator(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "agentic-log-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := Config{
		Level:          "DEBUG",
		ConsoleEnabled: false,
		FileEnabled:    true,
		Dir:            tempDir,
		FileName:       "test.log",
		MaxSizeMB:      1,
		MaxBackups:     3,
	}

	l, err := Init(cfg)
	if err != nil {
		t.Fatalf("failed to init logger: %v", err)
	}

	l.Info("hello world", slog.String("key", "value"))

	// Verify log file exists and contains expected content
	logPath := filepath.Join(tempDir, "test.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	var entry map[string]any
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("failed to parse JSON log entry: %v", err)
	}

	if entry["msg"] != "hello world" {
		t.Errorf("expected msg 'hello world', got %v", entry["msg"])
	}
	if entry["key"] != "value" {
		t.Errorf("expected key 'value', got %v", entry["key"])
	}
	if entry["level"] != "INFO" {
		t.Errorf("expected level 'INFO', got %v", entry["level"])
	}
	if entry["source"] == nil {
		t.Error("expected source details to be present")
	}

	// Direct test of rotator rotation
	rot, err := NewLogRotator(tempDir, "rotate.log", 1, 2)
	if err != nil {
		t.Fatalf("failed to create rotator: %v", err)
	}
	defer rot.Close()

	// override maxSize to a tiny value to trigger rotation
	rot.maxSize = 20

	// Write first log (smaller than 20 bytes)
	_, err = rot.Write([]byte("line 1\n"))
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Write second log (triggers rotation)
	_, err = rot.Write([]byte("line 2: this is a long line that will exceed 20 bytes\n"))
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Check that a backup file was created
	files, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("read dir failed: %v", err)
	}

	var backupFound bool
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "rotate.") && f.Name() != "rotate.log" {
			backupFound = true
			backupPath := filepath.Join(tempDir, f.Name())
			content, err := os.ReadFile(backupPath)
			if err != nil {
				t.Fatalf("failed to read backup file: %v", err)
			}
			if string(content) != "line 1\n" {
				t.Errorf("expected backup to contain 'line 1\n', got %q", string(content))
			}
		}
	}
	if !backupFound {
		t.Error("expected to find a backup log file")
	}
}

func TestCustomLoggerCallerSource(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "agentic-log-caller-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := Config{
		Level:          "DEBUG",
		ConsoleEnabled: false,
		FileEnabled:    true,
		Dir:            tempDir,
		FileName:       "caller.log",
		MaxSizeMB:      1,
		MaxBackups:     1,
	}

	l, err := Init(cfg)
	if err != nil {
		t.Fatalf("failed to init logger: %v", err)
	}

	cl := NewCustomLogger(l, "test-module")
	cl.Infof("test log with formatting: %s", "hello")

	logPath := filepath.Join(tempDir, "caller.log")
	file, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("failed to open log file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		t.Fatalf("no log lines found")
	}

	var entry map[string]any
	if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse JSON log entry: %v", err)
	}

	if entry["msg"] != "test log with formatting: hello" {
		t.Errorf("expected msg, got %v", entry["msg"])
	}
	if entry["module"] != "test-module" {
		t.Errorf("expected module 'test-module', got %v", entry["module"])
	}

	source, ok := entry["source"].(map[string]any)
	if !ok {
		t.Fatalf("source not found or not a map: %v", entry["source"])
	}

	// The function should be TestCustomLoggerCallerSource, not the wrapper helper log method
	function := source["function"].(string)
	if !strings.Contains(function, "TestCustomLoggerCallerSource") {
		t.Errorf("expected calling function to contain 'TestCustomLoggerCallerSource', got %q", function)
	}
}
