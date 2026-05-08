package logger

import (
	"testing"

	"go.uber.org/zap/zapcore"
)

func TestParseLevel(t *testing.T) {
	cases := map[string]zapcore.Level{
		"":      zapcore.InfoLevel,
		"info":  zapcore.InfoLevel,
		"debug": zapcore.DebugLevel,
		"warn":  zapcore.WarnLevel,
		"error": zapcore.ErrorLevel,
		"DEBUG": zapcore.DebugLevel,
	}
	for in, want := range cases {
		got, err := parseLevel(in)
		if err != nil {
			t.Errorf("parseLevel(%q) errored: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("parseLevel(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestParseLevel_Invalid(t *testing.T) {
	if _, err := parseLevel("nonsense"); err == nil {
		t.Fatal("expected error for invalid level, got nil")
	}
}

func TestConfigure_AcceptsKnownFormats(t *testing.T) {
	if err := Configure("info", "console"); err != nil {
		t.Fatalf("console: %v", err)
	}
	if err := Configure("debug", "json"); err != nil {
		t.Fatalf("json: %v", err)
	}
	if err := Configure("info", ""); err != nil {
		t.Fatalf("empty format should default to console: %v", err)
	}
}

func TestConfigure_RejectsUnknownFormat(t *testing.T) {
	if err := Configure("info", "xml"); err == nil {
		t.Fatal("expected error for unknown format, got nil")
	}
}
