package logging

import (
	"fmt"
	"log/slog"
	"strings"
)

// ParseLevel converts a string level to slog.Level.
func ParseLevel(value string) (slog.Level, error) {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "", "INFO":
		return slog.LevelInfo, nil
	case "DEBUG":
		return slog.LevelDebug, nil
	case "WARN", "WARNING":
		return slog.LevelWarn, nil
	case "ERROR":
		return slog.LevelError, nil
	case "TRACE":
		return slog.LevelDebug - 4, nil
	case "FATAL":
		return slog.LevelError + 4, nil
	default:
		return 0, fmt.Errorf("unsupported log level %q", value)
	}
}
