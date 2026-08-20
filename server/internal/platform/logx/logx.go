package logx

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

var sensitiveFragments = []string{"authorization", "cookie", "password", "passwd", "secret", "token", "api_key", "apikey", "private_key", "credential"}

type RedactingHandler struct{ next slog.Handler }

func New(level, format, service, env, version string) *slog.Logger {
	lvl := slog.LevelInfo
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	}
	opts := &slog.HandlerOptions{Level: lvl}
	var base slog.Handler
	if strings.EqualFold(format, "text") {
		base = slog.NewTextHandler(os.Stdout, opts)
	} else {
		base = slog.NewJSONHandler(os.Stdout, opts)
	}
	return slog.New(RedactingHandler{next: base}).With("service", service, "environment", env, "version", version)
}

func (h RedactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}
func (h RedactingHandler) Handle(ctx context.Context, r slog.Record) error {
	clean := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool { clean.AddAttrs(redact(a)); return true })
	return h.next.Handle(ctx, clean)
}
func (h RedactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		out[i] = redact(a)
	}
	return RedactingHandler{next: h.next.WithAttrs(out)}
}
func (h RedactingHandler) WithGroup(name string) slog.Handler {
	return RedactingHandler{next: h.next.WithGroup(name)}
}
func redact(a slog.Attr) slog.Attr {
	key := strings.ToLower(strings.ReplaceAll(a.Key, "-", "_"))
	for _, fragment := range sensitiveFragments {
		if strings.Contains(key, fragment) {
			return slog.String(a.Key, "[REDACTED]")
		}
	}
	if a.Value.Kind() == slog.KindGroup {
		xs := a.Value.Group()
		for i := range xs {
			xs[i] = redact(xs[i])
		}
		return slog.Group(a.Key, anyAttrs(xs)...)
	}
	return a
}
func anyAttrs(xs []slog.Attr) []any {
	out := make([]any, len(xs))
	for i := range xs {
		out[i] = xs[i]
	}
	return out
}
