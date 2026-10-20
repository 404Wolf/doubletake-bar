// Package logging provides the process-wide zap logger. doubletake-bar is a
// background GTK app launched from a waybar on-click handler, with no
// attached terminal most of the time, so it logs to a file under
// XDG_STATE_HOME instead of relying on stderr being seen.
package logging

import (
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New builds a logger that writes to ~/.local/state/doubletake-bar/<name>.log
// and, when stderr is a terminal, also to stderr.
func New(name string) *zap.Logger {
	cfg := zap.NewProductionEncoderConfig()
	cfg.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.EncodeLevel = zapcore.CapitalLevelEncoder
	encoder := zapcore.NewConsoleEncoder(cfg)

	cores := []zapcore.Core{}

	if path := logFilePath(name); path != "" {
		if f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
			cores = append(cores, zapcore.NewCore(encoder, zapcore.AddSync(f), zapcore.DebugLevel))
		}
	}
	if isTerminal(os.Stderr) {
		cores = append(cores, zapcore.NewCore(encoder, zapcore.AddSync(os.Stderr), zapcore.DebugLevel))
	}
	if len(cores) == 0 {
		cores = append(cores, zapcore.NewCore(encoder, zapcore.AddSync(os.Stderr), zapcore.DebugLevel))
	}

	return zap.New(zapcore.NewTee(cores...))
}

func logFilePath(name string) string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(home, ".local", "state")
	}
	dir = filepath.Join(dir, "doubletake-bar")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ""
	}
	return filepath.Join(dir, name+".log")
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}
