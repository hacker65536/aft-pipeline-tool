package utils

import (
	"fmt"
	"os"
	"runtime"
)

// ANSI color codes
const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	Purple = "\033[35m"
	Cyan   = "\033[36m"
	White  = "\033[37m"

	// Bright colors
	BrightRed    = "\033[91m"
	BrightGreen  = "\033[92m"
	BrightYellow = "\033[93m"
	BrightBlue   = "\033[94m"
	BrightPurple = "\033[95m"
	BrightCyan   = "\033[96m"
	BrightWhite  = "\033[97m"
)

// ColorOutput controls whether colors are enabled
var ColorOutput = true

// init initializes color output based on environment
func init() {
	// Disable colors on Windows by default (unless explicitly enabled)
	if runtime.GOOS == "windows" {
		ColorOutput = false
	}

	// Check if NO_COLOR environment variable is set
	if os.Getenv("NO_COLOR") != "" {
		ColorOutput = false
	}

	// Check if FORCE_COLOR environment variable is set
	if os.Getenv("FORCE_COLOR") != "" {
		ColorOutput = true
	}
}

// Colorize applies color to text if colors are enabled
func Colorize(color, text string) string {
	if !ColorOutput {
		return text
	}
	return color + text + Reset
}

// Info formats text with info color (cyan)
func Info(text string) string {
	return Colorize(Cyan, text)
}

// Success formats text with success color (green)
func Success(text string) string {
	return Colorize(Green, text)
}

// Warning formats text with warning color (yellow)
func Warning(text string) string {
	return Colorize(Yellow, text)
}

// Error formats text with error color (red)
func Error(text string) string {
	return Colorize(Red, text)
}

// Debug formats text with debug color (purple)
func Debug(text string) string {
	return Colorize(Purple, text)
}

// Highlight formats text with highlight color (bright blue)
func Highlight(text string) string {
	return Colorize(BrightBlue, text)
}

// InfoPrintf prints formatted info message with color
func InfoPrintf(format string, args ...interface{}) {
	fmt.Printf(Info("INFO: ")+format+"\n", args...)
}

// SuccessPrintf prints formatted success message with color
func SuccessPrintf(format string, args ...interface{}) {
	fmt.Printf(Success("SUCCESS: ")+format+"\n", args...)
}

// WarningPrintf prints formatted warning message with color
func WarningPrintf(format string, args ...interface{}) {
	fmt.Printf(Warning("WARNING: ")+format+"\n", args...)
}

// ErrorPrintf prints formatted error message with color
func ErrorPrintf(format string, args ...interface{}) {
	fmt.Printf(Error("ERROR: ")+format+"\n", args...)
}

// DebugPrintf prints formatted debug message with color
func DebugPrintf(format string, args ...interface{}) {
	fmt.Printf(Debug("DEBUG: ")+format+"\n", args...)
}

// SetColorOutput enables or disables color output
func SetColorOutput(enabled bool) {
	ColorOutput = enabled
}

// IsColorEnabled returns whether color output is enabled
func IsColorEnabled() bool {
	return ColorOutput
}
