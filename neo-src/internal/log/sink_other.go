//go:build !android

package log

import (
	"log/slog"
	"os"
)

func initSink(file *os.File) slog.Handler {
	return &textHandler{
		Handler: slog.NewTextHandler(file, nil),
		file:    file,
	}
}