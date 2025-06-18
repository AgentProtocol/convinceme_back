package logging

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// LogLevel represents the severity of a log message
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

// String returns the string representation of the log level
func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// Color codes for terminal output
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"
	ColorGray   = "\033[37m"
	ColorWhite  = "\033[97m"
)

// Logger provides structured logging capabilities
type Logger struct {
	level      LogLevel
	prefix     string
	colored    bool
	fileLogger *log.Logger
	file       *os.File
}

// Global logger instance
var defaultLogger *Logger

// Config holds logger configuration
type Config struct {
	Level       LogLevel
	Prefix      string
	Colored     bool
	LogToFile   bool
	LogFilePath string
}

// NewLogger creates a new logger instance
func NewLogger(config Config) (*Logger, error) {
	logger := &Logger{
		level:   config.Level,
		prefix:  config.Prefix,
		colored: config.Colored,
	}

	// Set up file logging if enabled
	if config.LogToFile {
		if config.LogFilePath == "" {
			config.LogFilePath = "logs/app.log"
		}

		// Create logs directory if it doesn't exist
		logDir := filepath.Dir(config.LogFilePath)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %v", err)
		}

		// Open log file
		file, err := os.OpenFile(config.LogFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %v", err)
		}

		logger.file = file
		logger.fileLogger = log.New(file, "", 0)
	}

	return logger, nil
}

// InitDefaultLogger initializes the global logger
func InitDefaultLogger(config Config) error {
	var err error
	defaultLogger, err = NewLogger(config)
	return err
}

// Close closes the logger and any open files
func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// formatMessage formats a log message with timestamp, level, caller info, and message
func (l *Logger) formatMessage(level LogLevel, msg string, context map[string]interface{}) string {
	// Get caller information
	_, file, line, ok := runtime.Caller(3)
	var caller string
	if ok {
		caller = fmt.Sprintf("%s:%d", filepath.Base(file), line)
	} else {
		caller = "unknown"
	}

	// Format timestamp
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")

	// Format context
	var contextStr string
	if len(context) > 0 {
		var pairs []string
		for k, v := range context {
			pairs = append(pairs, fmt.Sprintf("%s=%v", k, v))
		}
		contextStr = fmt.Sprintf(" [%s]", strings.Join(pairs, " "))
	}

	// Create base message
	baseMsg := fmt.Sprintf("[%s] %s %s %s%s",
		timestamp,
		level.String(),
		caller,
		msg,
		contextStr,
	)

	// Add color if enabled and outputting to terminal
	if l.colored {
		var color string
		switch level {
		case DEBUG:
			color = ColorGray
		case INFO:
			color = ColorBlue
		case WARN:
			color = ColorYellow
		case ERROR:
			color = ColorRed
		case FATAL:
			color = ColorPurple
		}

		if color != "" {
			baseMsg = color + baseMsg + ColorReset
		}
	}

	// Add prefix if set
	if l.prefix != "" {
		baseMsg = fmt.Sprintf("[%s] %s", l.prefix, baseMsg)
	}

	return baseMsg
}

// log is the internal logging method
func (l *Logger) log(level LogLevel, msg string, context map[string]interface{}) {
	if level < l.level {
		return
	}

	formattedMsg := l.formatMessage(level, msg, context)

	// Log to console
	fmt.Println(formattedMsg)

	// Log to file if configured
	if l.fileLogger != nil {
		// Remove color codes for file output
		fileMsg := l.formatMessage(level, msg, context)
		if l.colored {
			// Strip ANSI color codes for file output
			fileMsg = strings.ReplaceAll(fileMsg, ColorReset, "")
			fileMsg = strings.ReplaceAll(fileMsg, ColorRed, "")
			fileMsg = strings.ReplaceAll(fileMsg, ColorYellow, "")
			fileMsg = strings.ReplaceAll(fileMsg, ColorBlue, "")
			fileMsg = strings.ReplaceAll(fileMsg, ColorPurple, "")
			fileMsg = strings.ReplaceAll(fileMsg, ColorCyan, "")
			fileMsg = strings.ReplaceAll(fileMsg, ColorGray, "")
			fileMsg = strings.ReplaceAll(fileMsg, ColorWhite, "")
		}
		l.fileLogger.Println(fileMsg)
	}

	// Exit on fatal
	if level == FATAL {
		os.Exit(1)
	}
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, context ...map[string]interface{}) {
	ctx := mergeContext(context...)
	l.log(DEBUG, msg, ctx)
}

// Info logs an info message
func (l *Logger) Info(msg string, context ...map[string]interface{}) {
	ctx := mergeContext(context...)
	l.log(INFO, msg, ctx)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, context ...map[string]interface{}) {
	ctx := mergeContext(context...)
	l.log(WARN, msg, ctx)
}

// Error logs an error message
func (l *Logger) Error(msg string, context ...map[string]interface{}) {
	ctx := mergeContext(context...)
	l.log(ERROR, msg, ctx)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(msg string, context ...map[string]interface{}) {
	ctx := mergeContext(context...)
	l.log(FATAL, msg, ctx)
}

// Convenience functions for global logger
func Debug(msg string, context ...map[string]interface{}) {
	if defaultLogger != nil {
		defaultLogger.Debug(msg, context...)
	}
}

func Info(msg string, context ...map[string]interface{}) {
	if defaultLogger != nil {
		defaultLogger.Info(msg, context...)
	}
}

func Warn(msg string, context ...map[string]interface{}) {
	if defaultLogger != nil {
		defaultLogger.Warn(msg, context...)
	}
}

func Error(msg string, context ...map[string]interface{}) {
	if defaultLogger != nil {
		defaultLogger.Error(msg, context...)
	}
}

func Fatal(msg string, context ...map[string]interface{}) {
	if defaultLogger != nil {
		defaultLogger.Fatal(msg, context...)
	}
}

// mergeContext merges multiple context maps into one
func mergeContext(contexts ...map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for _, ctx := range contexts {
		for k, v := range ctx {
			result[k] = v
		}
	}
	return result
}

// LogWebSocketEvent logs WebSocket events with standardized format
func LogWebSocketEvent(event string, debateID string, clientID string, details map[string]interface{}) {
	context := map[string]interface{}{
		"event":     event,
		"debate_id": debateID,
		"client_id": clientID,
	}
	for k, v := range details {
		context[k] = v
	}
	Info("WebSocket Event", context)
}

// LogDebateEvent logs debate-related events
func LogDebateEvent(event string, debateID string, details map[string]interface{}) {
	context := map[string]interface{}{
		"event":     event,
		"debate_id": debateID,
	}
	for k, v := range details {
		context[k] = v
	}
	Info("Debate Event", context)
}

// LogAgentEvent logs agent-related events
func LogAgentEvent(event string, agentName string, debateID string, details map[string]interface{}) {
	context := map[string]interface{}{
		"event":      event,
		"agent_name": agentName,
		"debate_id":  debateID,
	}
	for k, v := range details {
		context[k] = v
	}
	Info("Agent Event", context)
}

// LogScoreEvent logs scoring-related events
func LogScoreEvent(event string, debateID string, scores map[string]interface{}) {
	context := map[string]interface{}{
		"event":     event,
		"debate_id": debateID,
	}
	for k, v := range scores {
		context[k] = v
	}
	Info("Score Event", context)
}

// LogTTSEvent logs text-to-speech events
func LogTTSEvent(event string, agentName string, details map[string]interface{}) {
	context := map[string]interface{}{
		"event":      event,
		"agent_name": agentName,
	}
	for k, v := range details {
		context[k] = v
	}
	Info("TTS Event", context)
}

// LogHTTPRequest logs HTTP requests
func LogHTTPRequest(method string, path string, statusCode int, duration time.Duration, details map[string]interface{}) {
	context := map[string]interface{}{
		"method":      method,
		"path":        path,
		"status_code": statusCode,
		"duration_ms": duration.Milliseconds(),
	}
	for k, v := range details {
		context[k] = v
	}
	Info("HTTP Request", context)
}

// LogDatabaseEvent logs database operations
func LogDatabaseEvent(operation string, table string, details map[string]interface{}) {
	context := map[string]interface{}{
		"operation": operation,
		"table":     table,
	}
	for k, v := range details {
		context[k] = v
	}
	Debug("Database Event", context)
}

// GetDefaultLogger returns the default logger instance
func GetDefaultLogger() *Logger {
	return defaultLogger
}
