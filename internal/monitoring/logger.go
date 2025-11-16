package monitoring

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// LogLevel represents the severity level of a log message
type LogLevel int

const (
	DebugLevel LogLevel = iota
	InfoLevel
	WarnLevel
	ErrorLevel
	FatalLevel
)

// String returns the string representation of a log level
func (l LogLevel) String() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	case FatalLevel:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// Logger provides structured logging capabilities
type Logger struct {
	mu          sync.Mutex
	level       LogLevel
	format      string // "json" or "text"
	output      io.Writer
	fields      map[string]interface{}
}

// NewLogger creates a new Logger instance
func NewLogger(level string, format string) *Logger {
	var logLevel LogLevel
	switch level {
	case "debug":
		logLevel = DebugLevel
	case "info":
		logLevel = InfoLevel
	case "warn":
		logLevel = WarnLevel
	case "error":
		logLevel = ErrorLevel
	case "fatal":
		logLevel = FatalLevel
	default:
		logLevel = InfoLevel
	}

	return &Logger{
		level:  logLevel,
		format: format,
		output: os.Stdout,
		fields: make(map[string]interface{}),
	}
}

// WithField adds a field to the logger context
func (l *Logger) WithField(key string, value interface{}) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()

	newLogger := &Logger{
		level:  l.level,
		format: l.format,
		output: l.output,
		fields: make(map[string]interface{}),
	}

	// Copy existing fields
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	newLogger.fields[key] = value

	return newLogger
}

// WithFields adds multiple fields to the logger context
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()

	newLogger := &Logger{
		level:  l.level,
		format: l.format,
		output: l.output,
		fields: make(map[string]interface{}),
	}

	// Copy existing fields
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	// Add new fields
	for k, v := range fields {
		newLogger.fields[k] = v
	}

	return newLogger
}

// WithError adds an error to the logger context
func (l *Logger) WithError(err error) *Logger {
	if err == nil {
		return l
	}
	return l.WithField("error", err.Error())
}

// Debug logs a debug message
func (l *Logger) Debug(msg string) {
	l.log(DebugLevel, msg, nil)
}

// Debugf logs a formatted debug message
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.log(DebugLevel, fmt.Sprintf(format, args...), nil)
}

// Info logs an info message
func (l *Logger) Info(msg string) {
	l.log(InfoLevel, msg, nil)
}

// Infof logs a formatted info message
func (l *Logger) Infof(format string, args ...interface{}) {
	l.log(InfoLevel, fmt.Sprintf(format, args...), nil)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string) {
	l.log(WarnLevel, msg, nil)
}

// Warnf logs a formatted warning message
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.log(WarnLevel, fmt.Sprintf(format, args...), nil)
}

// Error logs an error message
func (l *Logger) Error(msg string) {
	l.log(ErrorLevel, msg, nil)
}

// Errorf logs a formatted error message
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.log(ErrorLevel, fmt.Sprintf(format, args...), nil)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(msg string) {
	l.log(FatalLevel, msg, nil)
	os.Exit(1)
}

// Fatalf logs a formatted fatal message and exits
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.log(FatalLevel, fmt.Sprintf(format, args...), nil)
	os.Exit(1)
}

// log writes a log entry
func (l *Logger) log(level LogLevel, msg string, err error) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	entry := LogEntry{
		Timestamp: time.Now().UTC(),
		Level:     level.String(),
		Message:   msg,
		Fields:    l.fields,
	}

	if err != nil {
		entry.Error = err.Error()
	}

	var output string
	if l.format == "json" {
		jsonBytes, _ := json.Marshal(entry)
		output = string(jsonBytes) + "\n"
	} else {
		output = l.formatText(entry)
	}

	l.output.Write([]byte(output))
}

// formatText formats a log entry as text
func (l *Logger) formatText(entry LogEntry) string {
	output := fmt.Sprintf("[%s] %s: %s",
		entry.Timestamp.Format(time.RFC3339),
		entry.Level,
		entry.Message,
	)

	if len(entry.Fields) > 0 {
		output += " |"
		for k, v := range entry.Fields {
			output += fmt.Sprintf(" %s=%v", k, v)
		}
	}

	if entry.Error != "" {
		output += fmt.Sprintf(" | error=%s", entry.Error)
	}

	return output + "\n"
}

// Global logger instance
var globalLogger = NewLogger("info", "json")

// SetGlobalLogger sets the global logger instance
func SetGlobalLogger(logger *Logger) {
	globalLogger = logger
}

// GetLogger returns the global logger instance
func GetLogger() *Logger {
	return globalLogger
}

// Convenience functions for global logger
func Debug(msg string) {
	globalLogger.Debug(msg)
}

func Debugf(format string, args ...interface{}) {
	globalLogger.Debugf(format, args...)
}

func Info(msg string) {
	globalLogger.Info(msg)
}

func Infof(format string, args ...interface{}) {
	globalLogger.Infof(format, args...)
}

func Warn(msg string) {
	globalLogger.Warn(msg)
}

func Warnf(format string, args ...interface{}) {
	globalLogger.Warnf(format, args...)
}

func Error(msg string) {
	globalLogger.Error(msg)
}

func Errorf(format string, args ...interface{}) {
	globalLogger.Errorf(format, args...)
}

func Fatal(msg string) {
	globalLogger.Fatal(msg)
}

func Fatalf(format string, args ...interface{}) {
	globalLogger.Fatalf(format, args...)
}

func WithField(key string, value interface{}) *Logger {
	return globalLogger.WithField(key, value)
}

func WithFields(fields map[string]interface{}) *Logger {
	return globalLogger.WithFields(fields)
}

func WithError(err error) *Logger {
	return globalLogger.WithError(err)
}
