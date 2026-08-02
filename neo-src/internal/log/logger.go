package log

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"log/slog"
)

var logger *slog.Logger = slog.New(slog.NewTextHandler(os.Stderr, nil))

type textHandler struct {
	slog.Handler
	file *os.File
}

func (h *textHandler) Handle(ctx context.Context, r slog.Record) error {
	timestamp := r.Time.Format("2006-01-02T15:04:05Z")
	msg := r.Message
	level := r.Level.String()
	r.Attrs(func(attr slog.Attr) bool {
		fmt.Fprintf(h.file, " %s=%s", attr.Key, attr.Value.String())
		return true
	})
	fmt.Fprintln(h.file, formatLine(timestamp, level, msg))
	return nil
}

func formatLine(timestamp, level, msg string) string {
	return fmt.Sprintf("[%s] [%s]  %s", timestamp, level, msg)
}

func (h *textHandler) WithGroup(name string) slog.Handler {
	h.Handler = h.Handler.WithGroup(name)
	return h
}

func openLogDir(dir string) (*os.File, error) {
	if dir == "" {
		return os.Stderr, nil
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool {
		info, err := files[i].Info()
		if err != nil {
			return true
		}
		infoJ, err := files[j].Info()
		if err != nil {
			return false
		}
		return info.Size() > infoJ.Size()
	})
	var tmpl *os.File
	if len(files) > 0 {
		tmpl, err = os.OpenFile(filepath.Join(dir, files[len(files)-1].Name()), os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, err
		}
		info, err := tmpl.Stat()
		if err != nil {
			return nil, err
		}
		if info.Size() > 10*1024*1024 {
			tmpl.Close()
			tmpl = nil
		}
	}
	if tmpl == nil {
		t := time.Now()
		filename := filepath.Join(dir, t.Format("2006-01-02_15-04-05")+".log")
		tmpl, err = os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			return nil, err
		}
	}
	return tmpl, nil
}

func Init(logDir string) error {
	if logDir == "" {
		return nil
	}
	file, err := openLogDir(logDir)
	if err != nil {
		return err
	}
	logger = slog.New(initSink(file))
	return nil
}

func Debug(msg string, args ...any) {
	logger.Debug(msg, "args", args)
}

func Info(msg string, args ...any) {
	logger.Info(msg, "args", args)
}

func Warn(msg string, args ...any) {
	logger.Warn(msg, "args", args)
}

func Error(msg string, args ...any) {
	logger.Error(msg, "args", args)
}
