package log

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

func parseLogFile(prefix, name string) (int, bool) {
	if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".log") {
		return 0, false
	}
	middle := strings.TrimSuffix(name[len(prefix):], ".log")
	if middle == "" {
		return 0, true
	}
	if len(middle) < 2 || middle[0] != '.' {
		return 0, false
	}
	n, err := strconv.Atoi(middle[1:])
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

func openLogDir(dir string) (*os.File, error) {
	if dir == "" {
		return os.Stderr, nil
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	t := time.Now()
	prefix := t.Format("2006-01-02")
	const maxSize = 10 * 1024 * 1024

	var (
		baseInfo         os.FileInfo
		baseExists       bool
		bestSuffix       int
		bestSuffixExists bool
		highestSuffix    int
	)

	for _, entry := range entries {
		name := entry.Name()
		suffix, ok := parseLogFile(prefix, name)
		if !ok {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if suffix == 0 {
			baseExists = true
			baseInfo = info
		} else {
			if suffix > highestSuffix {
				highestSuffix = suffix
			}
			if info.Size() <= maxSize && suffix > bestSuffix {
				bestSuffix = suffix
				bestSuffixExists = true
			}
		}
	}

	// Preferred: base file under size cap.
	if baseExists && baseInfo.Size() <= maxSize {
		return os.OpenFile(filepath.Join(dir, prefix+".log"), os.O_WRONLY|os.O_APPEND, 0644)
	}

	// Fallback: highest suffix file under size cap.
	if bestSuffixExists {
		return os.OpenFile(filepath.Join(dir, prefix+"."+strconv.Itoa(bestSuffix)+".log"), os.O_WRONLY|os.O_APPEND, 0644)
	}

	// Create new file.
	var filename string
	if !baseExists && highestSuffix == 0 {
		filename = prefix + ".log"
	} else {
		next := highestSuffix + 1
		if baseExists && next == 1 {
			next = 2
		}
		filename = prefix + "." + strconv.Itoa(next) + ".log"
	}
	return os.OpenFile(filepath.Join(dir, filename), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
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
