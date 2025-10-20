package obs

import (
	"log/slog"
	"os"
)

var Logger *slog.Logger

func InitLogger() {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	Logger = slog.New(h)
}
