package logctx

import (
	"context"

	"github.com/sirupsen/logrus"
)

var logEntryKey = struct{ logEntryKey string }{}

// GetLogEntry gets the log entry from the context or returns a default entry.
func GetLogEntry(ctx context.Context) *logrus.Entry {
	loggerInter := ctx.Value(&logEntryKey)
	if loggerInter != nil {
		return loggerInter.(*logrus.Entry)
	}

	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)
	return logrus.NewEntry(log)
}

// WithLogEntry builds a context with a log entry.
func WithLogEntry(ctx context.Context, le *logrus.Entry) context.Context {
	return context.WithValue(ctx, &logEntryKey, le)
}
