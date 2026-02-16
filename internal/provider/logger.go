package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Logger adapts the go-retryablehttp LeveledLogger interface to Terraform's tflog system.
//
// go-retryablehttp (the HTTP client library we use) wants to log things like retry
// attempts, request timing, etc. Rather than printing to stderr, we route these through
// tflog so they appear in Terraform's structured log output (visible with TF_LOG=DEBUG).
//
// The LeveledLogger interface expects methods like Error(msg, key, val, key, val...)
// where keys and values alternate. We convert those to a map for tflog.
type Logger struct {
	ctx context.Context
}

func NewLogger(ctx context.Context) *Logger {
	return &Logger{ctx: ctx}
}

func (l *Logger) Error(msg string, keysAndValues ...any) {
	fields, err := toFields(keysAndValues)
	if err != nil {
		tflog.Error(l.ctx, fmt.Sprintf("logger field error: %v", err))
		return
	}
	tflog.Error(l.ctx, msg, fields)
}

func (l *Logger) Info(msg string, keysAndValues ...any) {
	fields, err := toFields(keysAndValues)
	if err != nil {
		tflog.Error(l.ctx, fmt.Sprintf("logger field error: %v", err))
		return
	}
	tflog.Info(l.ctx, msg, fields)
}

func (l *Logger) Debug(msg string, keysAndValues ...any) {
	fields, err := toFields(keysAndValues)
	if err != nil {
		tflog.Error(l.ctx, fmt.Sprintf("logger field error: %v", err))
		return
	}
	tflog.Debug(l.ctx, msg, fields)
}

func (l *Logger) Warn(msg string, keysAndValues ...any) {
	fields, err := toFields(keysAndValues)
	if err != nil {
		tflog.Error(l.ctx, fmt.Sprintf("logger field error: %v", err))
		return
	}
	tflog.Warn(l.ctx, msg, fields)
}

// toFields converts the alternating key/value pairs used by LeveledLogger into
// a map[string]any that tflog accepts. For example:
//
//	keysAndValues = ["url", "https://...", "status", 200]
//	→ map[string]any{"url": "https://...", "status": 200}
func toFields(keysAndValues []any) (map[string]any, error) {
	fields := make(map[string]any, len(keysAndValues)/2)
	for i := 0; i < len(keysAndValues); i += 2 {
		if i+1 >= len(keysAndValues) {
			return nil, fmt.Errorf("missing value for key %v", keysAndValues[i])
		}
		key, ok := keysAndValues[i].(string)
		if !ok {
			return nil, fmt.Errorf("key %v is not a string", keysAndValues[i])
		}
		fields[key] = keysAndValues[i+1]
	}
	return fields, nil
}
