/*
 * Component: Logger
 * Block-UUID: 9333cb70-ef93-4d56-b780-994eea009a8c
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Package logger provides standardized logging utilities for the GSC CLI.
 * Language: Go
 * Created-at: 2026-02-02T05:47:00.123Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package logger

import (
	"fmt"
	"os"
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

// Info logs an informational message
func Info(message string, args ...interface{}) {
	fmt.Printf("%s[INFO]%s %s\n", ColorBlue, ColorReset, fmt.Sprintf(message, args...))
}

// Success logs a success message
func Success(message string, args ...interface{}) {
	fmt.Printf("%s[SUCCESS]%s %s\n", ColorGreen, ColorReset, fmt.Sprintf(message, args...))
}

// Warning logs a warning message
func Warning(message string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "%s[WARNING]%s %s\n", ColorYellow, ColorReset, fmt.Sprintf(message, args...))
}

// Error logs an error message
func Error(message string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "%s[ERROR]%s %s\n", ColorRed, ColorReset, fmt.Sprintf(message, args...))
}

// Fatal logs a fatal error and exits
func Fatal(message string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "%s[FATAL]%s %s\n", ColorRed, ColorReset, fmt.Sprintf(message, args...))
	os.Exit(1)
}

// Debug logs a debug message (only if DEBUG env var is set)
func Debug(message string, args ...interface{}) {
	if os.Getenv("DEBUG") == "true" {
		fmt.Printf("%s[DEBUG]%s %s\n", ColorPurple, ColorReset, fmt.Sprintf(message, args...))
	}
}
