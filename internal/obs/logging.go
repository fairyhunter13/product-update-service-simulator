// Package obs contains observability utilities such as logging.
package obs

import (
	"log/slog"
	"os"
)

// Logger is the global structured logger used by the service.
//
// Logger is exported to allow other packages to use it for logging.
var Logger *slog.Logger

// InitLogger initializes the global Logger with JSON handler at info level.
//
// InitLogger is exported to allow other packages to initialize the Logger.
func InitLogger() {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	Logger = slog.New(h)
}
