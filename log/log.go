package log

import (
	"fmt"
	"log/slog"
	"os"
	"time"
)

const slogTime = "time"

func NewLogger(timezone string) {
	loc := loadTimezoneFromString(timezone)

	replaceAttr := func(groups []string, attr slog.Attr) slog.Attr {
		if attr.Key == slogTime {
			stringSlogFormat := formatSlogTime(loc)
			return slog.Attr{Key: slogTime, Value: slog.StringValue(stringSlogFormat)}
		}
		return slog.Attr{Key: attr.Key, Value: attr.Value}
	}
	textHandlerLogger := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		ReplaceAttr: replaceAttr,
	})

	logger := slog.New(textHandlerLogger)
	slog.SetDefault(logger)
}

func loadTimezoneFromString(timezone string) *time.Location {
	if timezone != "" {
		loc, err := time.LoadLocation(timezone)
		if err != nil {
			panic(err)
		}
		return loc
	}
	return time.UTC
}

func formatSlogTime(loc *time.Location) string {
	now := time.Now().In(loc)
	return fmt.Sprintf("%02d:%02d:%02d %02d/%02d/%04d (%s)", now.Hour(), now.Minute(), now.Second(), now.Day(), now.Month(), now.Year(), now.Weekday())
}
