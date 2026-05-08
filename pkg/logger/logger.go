package logger

import (
	"fmt"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Format is the logger output format.
type Format string

const (
	FormatConsole Format = "console"
	FormatJSON    Format = "json"
)

// Init installs a sane bootstrap logger (info-level console). It is intended
// to be called before configuration is loaded so early-startup errors still
// surface. Once config is loaded, call Configure to apply the user's
// preferences.
func Init() error {
	return Configure("info", string(FormatConsole))
}

// Configure builds a logger from the given level/format strings and installs
// it as the Zap global. Empty strings fall back to defaults (info / console).
func Configure(level, format string) error {
	lvl, err := parseLevel(level)
	if err != nil {
		return err
	}

	switch normalizeFormat(format) {
	case FormatJSON:
		return buildJSON(lvl)
	case FormatConsole:
		return buildConsole(lvl)
	default:
		return fmt.Errorf("unknown logging format %q (want %q or %q)", format, FormatConsole, FormatJSON)
	}
}

func buildConsole(lvl zapcore.Level) error {
	cfg := zap.Config{
		Level:            zap.NewAtomicLevelAt(lvl),
		Encoding:         "console",
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "ts",
			LevelKey:       "level",
			CallerKey:      "caller",
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.CapitalColorLevelEncoder,
			EncodeTime:     zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05"),
			EncodeCaller:   zapcore.ShortCallerEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
		},
	}

	l, err := cfg.Build()
	if err != nil {
		return fmt.Errorf("build console logger: %w", err)
	}
	zap.ReplaceGlobals(l)
	return nil
}

func buildJSON(lvl zapcore.Level) error {
	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(lvl)

	l, err := cfg.Build()
	if err != nil {
		return fmt.Errorf("build json logger: %w", err)
	}
	zap.ReplaceGlobals(l)
	return nil
}

func parseLevel(s string) (zapcore.Level, error) {
	if s == "" {
		return zapcore.InfoLevel, nil
	}
	var lvl zapcore.Level
	if err := lvl.UnmarshalText([]byte(strings.ToLower(s))); err != nil {
		return zapcore.InfoLevel, fmt.Errorf("invalid log level %q: %w", s, err)
	}
	return lvl, nil
}

func normalizeFormat(s string) Format {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", string(FormatConsole):
		return FormatConsole
	case string(FormatJSON):
		return FormatJSON
	default:
		return Format(s)
	}
}
