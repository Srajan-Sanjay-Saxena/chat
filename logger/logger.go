package logger

import (
	"log/slog"
	"os"
)

var Log *slog.Logger

func Init() {

	file, err := os.OpenFile("app.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic("Failed to open log file: " + err.Error())
	}

	Log = slog.New(
		slog.NewJSONHandler(file, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}),
	)
}
