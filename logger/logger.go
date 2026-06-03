package logger

import (
	"log/slog"
	"os"
)

var Log *slog.Logger

func Init() {

	file, err := os.OpenFile("app.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		// Fall back to a console test logger instead of panicking so test
		// runners that cannot create files (e.g., CI or restricted envs)
		// will still have a valid logger.
		TestInit()
		return
	}

	Log = slog.New(
		slog.NewJSONHandler(file, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}),
	)
}

func TestInit() {
	Log = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}
