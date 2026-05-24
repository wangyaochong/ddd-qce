package infrastructure

import "log"

type AppLogger struct{}

func NewAppLogger() *AppLogger {
	return &AppLogger{}
}

func (l *AppLogger) Info(msg string, args ...interface{}) {
	log.Printf("[INFO] "+msg, args...)
}

func (l *AppLogger) Error(msg string, args ...interface{}) {
	log.Printf("[ERROR] "+msg, args...)
}

func (l *AppLogger) Debug(msg string, args ...interface{}) {
	log.Printf("[DEBUG] "+msg, args...)
}
