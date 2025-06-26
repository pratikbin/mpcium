package monitoring

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fystack/mpcium/pkg/logger"
)

// KeygenTimestamps holds the structured data for a single key generation event.
type KeygenTimestamps struct {
	WalletID       string    `json:"wallet_id"`
	NodeID         string    `json:"node_id"`
	KeyType        string    `json:"key_type"`
	StartTime      time.Time `json:"start_time"`
	CompletionTime time.Time `json:"completion_time"`
	InitDurationMs int64     `json:"init_duration_ms"`
	RunDurationMs  int64     `json:"run_duration_ms"`
}

var (
	logFile *os.File
	logOnce sync.Once
	logMux  sync.Mutex
)

// initLogFile initializes the log file for appending. It ensures this only happens once.
func initLogFile() {
	logOnce.Do(func() {
		logDir := "monitoring"
		if err := os.MkdirAll(logDir, 0755); err != nil {
			logger.Error("Failed to create monitoring directory", err)
			return
		}

		var err error
		logFile, err = os.OpenFile(filepath.Join(logDir, "keygen_times.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			logger.Error("Failed to open keygen log file", err)
		}
	})
}

// RecordKeygenCompletion marshals the timestamp data to JSON and writes it to the log file.
func RecordKeygenCompletion(data KeygenTimestamps) {
	initLogFile()
	if logFile == nil {
		return // Initialization failed
	}

	logMux.Lock()
	defer logMux.Unlock()

	line, err := json.Marshal(data)
	if err != nil {
		logger.Error("Failed to marshal keygen timestamp data", err)
		return
	}

	if _, err := logFile.Write(append(line, '\n')); err != nil {
		logger.Error("Failed to write to keygen log file", err)
	}
}

// Close ensures the log file is synced and closed gracefully.
func Close() {
	logMux.Lock()
	defer logMux.Unlock()

	if logFile != nil {
		logFile.Sync()
		logFile.Close()
	}
}
