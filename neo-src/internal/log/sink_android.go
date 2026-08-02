//go:build android

package log

/*
#cgo LDFLAGS: -llog

#include <android/log.h>
#include <stdlib.h>

static void lt_log(int prio, const char* msg) {
	__android_log_print(prio, "LittleTimer", "%s", msg);
}
*/
import "C"

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"unsafe"
)

// androidHandler writes log records to Android logcat via __android_log_print.
type androidHandler struct{}

func (h *androidHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *androidHandler) Handle(_ context.Context, r slog.Record) error {
	timestamp := r.Time.Format("2006-01-02T15:04:05Z")
	msg := r.Message
	level := r.Level.String()
	line := formatLine(timestamp, level, msg)
	prio := androidLogPriority(r.Level)
	cstr := C.CString(line)
	C.lt_log(C.int(prio), cstr)
	C.free(unsafe.Pointer(cstr))
	return nil
}

func (h *androidHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *androidHandler) WithGroup(_ string) slog.Handler       { return h }

func androidLogPriority(l slog.Level) int {
	switch {
	case l >= slog.LevelError:
		return 6 // ANDROID_LOG_ERROR
	case l >= slog.LevelWarn:
		return 5 // ANDROID_LOG_WARN
	case l >= slog.LevelInfo:
		return 4 // ANDROID_LOG_INFO
	default:
		return 3 // ANDROID_LOG_DEBUG
	}
}

// fileHandler replicates textHandler.Handle's output to a file.
type fileHandler struct {
	file *os.File
}

func (h *fileHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *fileHandler) Handle(_ context.Context, r slog.Record) error {
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

func (h *fileHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *fileHandler) WithGroup(_ string) slog.Handler       { return h }

// fanoutHandler writes to multiple handlers (logcat + file).
type fanoutHandler struct {
	handlers []slog.Handler
}

func (f *fanoutHandler) Enabled(ctx context.Context, l slog.Level) bool {
	for _, h := range f.handlers {
		if h.Enabled(ctx, l) {
			return true
		}
	}
	return false
}

func (f *fanoutHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, h := range f.handlers {
		if err := h.Handle(ctx, r); err != nil {
			return err
		}
	}
	return nil
}

func (f *fanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		handlers[i] = h.WithAttrs(attrs)
	}
	return &fanoutHandler{handlers: handlers}
}

func (f *fanoutHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		handlers[i] = h.WithGroup(name)
	}
	return &fanoutHandler{handlers: handlers}
}

// initSink returns a top-level fanout handler that writes to both logcat and file.
func initSink(file *os.File) slog.Handler {
	return &fanoutHandler{
		handlers: []slog.Handler{
			&androidHandler{},
			&fileHandler{file: file},
		},
	}
}