package log

import (
	"log"
	"os"
)

var logger = log.New(os.Stderr, "[ZEN] ", log.LstdFlags)

func Error(format string, args ...any) {
	logger.Printf("ERROR: "+format, args...)
}

func Debug(format string, args ...any) {
	logger.Printf("DEBUG: "+format, args...)
}
