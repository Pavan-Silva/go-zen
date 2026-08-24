package log

import (
	"log"
	"os"
)

// DefaultWriter is the destination for zen's internal error and debug logs.
var DefaultWriter = os.Stderr

var logger = log.New(DefaultWriter, "[ZEN] ", log.LstdFlags)

func Error(format string, args ...any) {
	logger.Printf("ERROR: "+format, args...)
}

func Debug(format string, args ...any) {
	logger.Printf("DEBUG: "+format, args...)
}
