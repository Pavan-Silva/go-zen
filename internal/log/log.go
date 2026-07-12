package log

import (
	"log"
	"os"
)

var DefaultWriter = os.Stderr

var logger = log.New(DefaultWriter, "[zen] ", log.LstdFlags)

func Error(format string, args ...any) {
	logger.Printf("ERROR: "+format, args...)
}

func Debug(format string, args ...any) {
	logger.Printf("DEBUG: "+format, args...)
}
