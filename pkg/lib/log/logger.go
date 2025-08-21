package log

import (
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog"
)

func NewLogger(cfg *Config) (*zerolog.Logger, error) {
	level, err := parseLogLevel(cfg.Level)
	if err != nil {
		return nil, fmt.Errorf("parse log level: %w", err)
	}

	switch cfg.Format {
	case LogFormatConsole:
		l := zerolog.New(zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		}).Level(level).With().Timestamp().Logger()
		return &l, nil
	default:
		l := zerolog.New(os.Stdout).With().Timestamp().Logger().Level(level)
		return &l, nil
	}
}

func parseLogLevel(level LogLevel) (zerolog.Level, error) {
	switch level {
	case LogLevelTrace:
		return zerolog.TraceLevel, nil
	case LogLevelDebug:
		return zerolog.DebugLevel, nil
	case LogLevelInfo:
		return zerolog.InfoLevel, nil
	case LogLevelWarn:
		return zerolog.WarnLevel, nil
	case LogLevelError:
		return zerolog.ErrorLevel, nil
	case LogLevelFatal:
		return zerolog.FatalLevel, nil
	default:
		return 0, fmt.Errorf("invalid log level: %s", level)
	}
}
