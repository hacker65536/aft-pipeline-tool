package logger

import (
	"go.uber.org/zap"
)

var Logger *zap.Logger

// InitLogger initializes the global logger
func InitLogger(debug bool) error {
	var config zap.Config

	if debug {
		config = zap.NewDevelopmentConfig()
	} else {
		config = zap.NewProductionConfig()
		config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	logger, err := config.Build()
	if err != nil {
		return err
	}

	Logger = logger
	return nil
}

// GetLogger returns the global logger instance
func GetLogger() *zap.Logger {
	if Logger == nil {
		// Fallback to a no-op logger if not initialized
		Logger = zap.NewNop()
	}
	return Logger
}

// Sync flushes any buffered log entries
func Sync() {
	if Logger != nil {
		if err := Logger.Sync(); err != nil {
			// Note: Logger.Sync() can return errors on some platforms (like Windows)
			// but these are often not critical, so we just ignore them silently
			// to avoid noise in the logs
			_ = err // Explicitly ignore the error
		}
	}
}
