package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"time"
)

// MultiHandler multiplexes logs to multiple handlers
type MultiHandler struct {
	handlers []slog.Handler
}

func NewMultiHandler(handlers ...slog.Handler) *MultiHandler {
	return &MultiHandler{handlers: handlers}
}

func (m *MultiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *MultiHandler) Handle(ctx context.Context, record slog.Record) error {
	for _, h := range m.handlers {
		if h.Enabled(ctx, record.Level) {
			if err := h.Handle(ctx, record); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *MultiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		next[i] = h.WithAttrs(attrs)
	}
	return &MultiHandler{handlers: next}
}

func (m *MultiHandler) WithGroup(name string) slog.Handler {
	next := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		next[i] = h.WithGroup(name)
	}
	return &MultiHandler{handlers: next}
}

type Config struct {
	Level          string
	ConsoleEnabled bool
	FileEnabled    bool
	Dir            string
	FileName       string
	MaxSizeMB      int
	MaxBackups     int
}

var Log *slog.Logger = slog.Default()

// Init configures and sets the global structured logger
func Init(cfg Config) (*slog.Logger, error) {
	var level slog.Level
	switch strings.ToUpper(cfg.Level) {
	case "DEBUG":
		level = slog.LevelDebug
	case "WARN", "WARNING":
		level = slog.LevelWarn
	case "ERROR":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: true, // Appends emitter file location to JSON
	}

	var handlers []slog.Handler

	if cfg.ConsoleEnabled {
		handlers = append(handlers, slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: level,
		}))
	}

	if cfg.FileEnabled {
		rotator, err := NewLogRotator(cfg.Dir, cfg.FileName, cfg.MaxSizeMB, cfg.MaxBackups)
		if err != nil {
			return nil, fmt.Errorf("failed to init rotator: %w", err)
		}
		handlers = append(handlers, slog.NewJSONHandler(rotator, opts))
	}

	if len(handlers) == 0 {
		handlers = append(handlers, slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	}

	Log = slog.New(NewMultiHandler(handlers...))
	slog.SetDefault(Log)

	return Log, nil
}

type CustomLogger struct {
	logger *slog.Logger
	module string
}

func NewCustomLogger(logger *slog.Logger, module string) *CustomLogger {
	return &CustomLogger{
		logger: logger,
		module: module,
	}
}

func (c *CustomLogger) log(level slog.Level, format string, args ...interface{}) {
	ctx := context.Background()
	if !c.logger.Handler().Enabled(ctx, level) {
		return
	}
	msg := fmt.Sprintf(format, args...)

	var pcs [10]uintptr
	n := runtime.Callers(1, pcs[:])
	var pc uintptr
	if n > 0 {
		frames := runtime.CallersFrames(pcs[:n])
		i := 0
		for {
			frame, more := frames.Next()
			// Exclude wrapper helper functions from tracing
			if !strings.Contains(frame.Function, "(*CustomLogger).") && !strings.Contains(frame.Function, "runtime.") {
				pc = pcs[i]
				break
			}
			i++
			if !more {
				break
			}
		}
	}

	// Create and write the record using the true call site PC
	r := slog.NewRecord(time.Now(), level, msg, pc)
	if c.module != "" {
		r.AddAttrs(slog.String("module", c.module))
	}
	_ = c.logger.Handler().Handle(ctx, r)
}

func (c *CustomLogger) Infof(format string, args ...interface{}) {
	c.log(slog.LevelInfo, format, args...)
}

func (c *CustomLogger) Errorf(format string, args ...interface{}) {
	c.log(slog.LevelError, format, args...)
}

func (c *CustomLogger) Debugf(format string, args ...interface{}) {
	c.log(slog.LevelDebug, format, args...)
}

func (c *CustomLogger) Warnf(format string, args ...interface{}) {
	c.log(slog.LevelWarn, format, args...)
}
