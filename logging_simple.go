package qzmq

import (
	"fmt"
	"log"
)

// Simple logging functions using standard log package

func logDebug(msg string, args ...interface{}) {
	if len(args) > 0 {
		log.Printf("[DEBUG] %s %v", msg, args)
	} else {
		log.Printf("[DEBUG] %s", msg)
	}
}

func logInfo(msg string, args ...interface{}) {
	if len(args) > 0 {
		log.Printf("[INFO] %s %v", msg, args)
	} else {
		log.Printf("[INFO] %s", msg)
	}
}

func logWarn(msg string, args ...interface{}) {
	if len(args) > 0 {
		log.Printf("[WARN] %s %v", msg, args)
	} else {
		log.Printf("[WARN] %s", msg)
	}
}

func logError(msg string, args ...interface{}) {
	if len(args) > 0 {
		log.Printf("[ERROR] %s %v", msg, args)
	} else {
		log.Printf("[ERROR] %s", msg)
	}
}

func logFatal(msg string, args ...interface{}) {
	if len(args) > 0 {
		log.Fatalf("[FATAL] %s %v", msg, args)
	} else {
		log.Fatalf("[FATAL] %s", msg)
	}
}

// formatFields formats key-value pairs for logging
func formatFields(args []interface{}) string {
	if len(args) == 0 {
		return ""
	}

	var result string
	for i := 0; i < len(args)-1; i += 2 {
		if i > 0 {
			result += ", "
		}
		result += fmt.Sprintf("%v=%v", args[i], args[i+1])
	}
	return result
}
