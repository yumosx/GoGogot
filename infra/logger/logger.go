package logger

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func Init(logLevel string) {
	console := zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: "15:04:05",
		FormatLevel: func(i interface{}) string {
			lvl := strings.ToUpper(fmt.Sprint(i))
			switch lvl {
			case "DEBUG":
				return "\033[1;34mDBG\033[0m"
			case "INFO":
				return "\033[1;36mINF\033[0m"
			case "WARN":
				return "\033[1;33mWRN\033[0m"
			case "ERROR":
				return "\033[1;31mERR\033[0m"
			case "FATAL":
				return "\033[1;35mFTL\033[0m"
			default:
				return lvl
			}
		},
		FormatMessage: func(i interface{}) string {
			return fmt.Sprintf("\033[1mgogogot\033[0m %s", i)
		},
	}

	level := parseLevel(logLevel)
	zerolog.SetGlobalLevel(level)
	zerolog.TimeFieldFormat = time.RFC3339

	log.Logger = zerolog.New(console).With().Timestamp().Logger()

	log.Info().Str("level", level.String()).Msg("logger initialized")
}

func parseLevel(s string) zerolog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return zerolog.DebugLevel
	case "warn", "warning":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}
