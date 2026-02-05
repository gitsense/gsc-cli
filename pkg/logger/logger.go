/**
 * Component: Logger
 * Block-UUID: 81480f81-067e-4318-bb39-74f8796924fe
 * Parent-UUID: 30644fbe-cfc4-4df3-9469-8699e31cb478
 * Version: 2.1.0
 * Description: Package logger provides standardized logging utilities for the GSC CLI. Implemented a Log Level system (Error, Warning, Info, Debug) replacing the previous boolean quiet mode. Added key-value pair formatting and Stderr output.
 * Language: Go
 * Created-at: 2026-02-05T00:38:08.372Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v2.0.0), GLM-4.7 (v2.1.0)
 */


package logger

import (
	"fmt"
	"os"
	"strings"
)

// Color codes for terminal output
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"
	ColorWhite  = "\033[37m"
)

// Level represents the severity of a log message
type Level int

const (
	LevelError   Level = iota // 0
	LevelWarning              // 1
	LevelInfo                 // 2
	LevelDebug                // 3
)

// currentLevel is the global log level threshold
var currentLevel = LevelWarning // Default: Quiet (only Errors and Warnings)

// SetLogLevel sets the global log level threshold
func SetLogLevel(level Level) {
	currentLevel = level
}

// formatMessage handles both simple messages and key-value pairs
// It prevents the %!(EXTRA string=...) bug by appending args to the message string
func formatMessage(message string, args ...interface{}) string {
	if len(args) == 0 {
		return message
	}

	// If args are key-value pairs, format them nicely
	var parts []string
	parts = append(parts, message)

	for i := 0; i < len(args); i += 2 {
		if i+1 < len(args) {
			parts = append(parts, fmt.Sprintf("%v=%v", args[i], args[i+1]))
		} else {
			// Odd number of args, append the last one as is
			parts = append(parts, fmt.Sprintf("%v", args[i]))
		}
	}

	return strings.Join(parts, " ")
}

// Info logs an informational message (Level 2)
func Info(message string, args ...interface{}) {
	if currentLevel >= LevelWarning {
		formatted := formatMessage(message, args...)
		fmt.Fprintf(os.Stderr, "%s[INFO]%s %s\n", ColorBlue, ColorReset, formatted)
	}
}

// Success logs a success message (Level 2)
func Success(message string, args ...interface{}) {
	if currentLevel >= LevelInfo {
		formatted := formatMessage(message, args...)
		fmt.Fprintf(os.Stderr, "%s[SUCCESS]%s %s\n", ColorGreen, ColorReset, formatted)
	}
}

// Warning logs a warning message (Level 1)
func Warning(message string, args ...interface{}) {
	// Warnings are always visible (Level 1 is default)
	formatted := formatMessage(message, args...)
	fmt.Fprintf(os.Stderr, "%s[WARNING]%s %s\n", ColorYellow, ColorReset, formatted)
}

// Error logs an error message (Level 0)
func Error(message string, args ...interface{}) {
	// Errors are always visible (Level 0)
	formatted := formatMessage(message, args...)
	fmt.Fprintf(os.Stderr, "%s[ERROR]%s %s\n", ColorRed, ColorReset, formatted)
}

// Fatal logs a fatal error and exits (Level 0)
func Fatal(message string, args ...interface{}) {
	formatted := formatMessage(message, args...)
	fmt.Fprintf(os.Stderr, "%s[FATAL]%s %s\n", ColorRed, ColorReset, formatted)
	os.Exit(1)
}

// Debug logs a debug message (Level 3)
func Debug(message string, args ...interface{}) {
	if currentLevel >= LevelDebug {
		formatted := formatMessage(message, args...)
		fmt.Fprintf(os.Stderr, "%s[DEBUG]%s %s\n", ColorPurple, ColorReset, formatted)
	}
}
