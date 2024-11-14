package types

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

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
	default:
		return "UNKNOWN"
	}
}

type LogEntry struct {
	Timestamp   time.Time `json:"timestamp"`
	Level       string    `json:"level"`
	Message     string    `json:"message"`
	Operation   string    `json:"operation,omitempty"`
	Source      string    `json:"source,omitempty"`
	Destination string    `json:"destination,omitempty"`
	Error       string    `json:"error,omitempty"`
	BytesCount  int64     `json:"bytes_count,omitempty"`
}

type Logger struct {
	LogFile  string
	LogLevel LogLevel
	file     *os.File
	metrics  *MetricsCollector
}

func NewLogger(logFile string, level LogLevel) (*Logger, error) {
	if err := os.MkdirAll(filepath.Dir(logFile), 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %v", err)
	}

	file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %v", err)
	}

	return &Logger{
		LogFile:  logFile,
		LogLevel: level,
		file:     file,
		metrics:  NewMetricsCollector(),
	}, nil
}

func (l *Logger) Log(level LogLevel, entry LogEntry) {
	if level < l.LogLevel {
		return
	}

	entry.Timestamp = time.Now()
	entry.Level = level.String()

	jsonEntry, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal log entry: %v\n", err)
		return
	}

	if _, err := l.file.Write(append(jsonEntry, '\n')); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write log entry: %v\n", err)
	}

	// Update metrics
	l.metrics.RecordOperation(entry)
}

func (l *Logger) Close() error {
	return l.file.Close()
}

func (l *Logger) LogError(message string) {
	l.Log(ERROR, LogEntry{
		Message: message,
	})
}

func (l *Logger) LogInfo(message string) {
	l.Log(INFO, LogEntry{
		Message: message,
	})
}

func (l *Logger) LogWarn(message string) {
	l.Log(WARN, LogEntry{
		Message: message,
	})
}

func (l *Logger) LogDebug(message string) {
	l.Log(DEBUG, LogEntry{
		Message: message,
	})
}
