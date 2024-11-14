package types

import (
	"sync"
	"time"
)

type MetricsCollector struct {
	mu sync.RWMutex

	TotalOperations    int64
	FailedOperations   int64
	BytesTransferred   int64
	OperationLatencies []time.Duration
	LastOperationTime  time.Time
	OperationsByType   map[string]int64
	ErrorsByType       map[string]int64
}

func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		OperationsByType: make(map[string]int64),
		ErrorsByType:     make(map[string]int64),
	}
}

func (m *MetricsCollector) RecordOperation(entry LogEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalOperations++
	if entry.Error != "" {
		m.FailedOperations++
		m.ErrorsByType[entry.Error]++
	}
	if entry.BytesCount > 0 {
		m.BytesTransferred += entry.BytesCount
	}
	m.OperationsByType[entry.Operation]++
	m.LastOperationTime = entry.Timestamp
}
