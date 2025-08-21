package log

type LogLevel string

const (
	LogLevelTrace LogLevel = "trace"
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
	LogLevelFatal LogLevel = "fatal"
)

type LogFormat string

const (
	LogFormatJSON    LogFormat = "json"
	LogFormatConsole LogFormat = "console"
)

type Config struct {
	Level  LogLevel  `env:"LOG_LEVEL,default=debug" validate:"required,oneof=trace debug info warn error fatal"`
	Format LogFormat `env:"LOG_FORMAT,default=json" validate:"required,oneof=json console"`
}
