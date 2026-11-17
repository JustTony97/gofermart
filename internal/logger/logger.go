package logger

import (
	"log/slog"
	"os"
)

var Log *slog.Logger

func Init() {
	handler := slog.NewTextHandler(os.Stdout, nil)
	Log = slog.New(handler)
	slog.SetDefault(Log)
}
