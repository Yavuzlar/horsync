package logger

import (
	"io"
	"log/slog"
	"os"

	"gopkg.in/natefinch/lumberjack.v2"
)

type Config struct {
	Level      slog.Level
	IsJSON     bool
	LogToFile  bool
	FilePath   string
	MaxSize    int
	MaxBackups int
	Service    string
}

var L *slog.Logger

func Init(cfg Config) {
	var writer io.Writer = os.Stdout

	if cfg.LogToFile {
		fileWriter := &lumberjack.Logger{
			Filename:   cfg.FilePath,
			MaxSize:    cfg.MaxSize,
			MaxBackups: cfg.MaxBackups,
			Compress:   true,
		}
		writer = io.MultiWriter(os.Stdout, fileWriter)
	}

	opts := &slog.HandlerOptions{
		Level: cfg.Level,
	}

	var handler slog.Handler
	if cfg.IsJSON {
		handler = slog.NewJSONHandler(writer, opts)
	} else {
		handler = slog.NewTextHandler(writer, opts)
	}

	L = slog.New(handler).With("service", cfg.Service)
}
