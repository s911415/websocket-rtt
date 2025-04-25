package main

import (
	"bytes"
	"fmt"
	"sync"
	"time"
)

// BufferedLogger provides buffered logging functionality
type BufferedLogger struct {
	buffer    bytes.Buffer
	mutex     sync.Mutex
	flushSize int
	interval  time.Duration
	stopChan  chan struct{}
}

// NewBufferedLogger creates a new buffered logger
func NewBufferedLogger(flushSize int, interval time.Duration) *BufferedLogger {
	bl := &BufferedLogger{
		flushSize: flushSize,
		interval:  interval,
		stopChan:  make(chan struct{}),
	}

	// Start periodic flushing
	go bl.periodicFlush()

	return bl
}

// Stop ends the periodic flushing goroutine
func (bl *BufferedLogger) Stop() {
	close(bl.stopChan)
	bl.Flush() // Final flush
}

// Write adds a log message to the buffer
func (bl *BufferedLogger) Write(msg string) {
	bl.mutex.Lock()
	defer bl.mutex.Unlock()

	bl.buffer.WriteString(fmt.Sprintf("[%s] %s\n", time.Now().UTC().Format("2006-01-02 15:04:05.000000"), msg))

	// Check if buffer size exceeds threshold
	if bl.buffer.Len() >= bl.flushSize {
		bl.flushLocked()
	}
}

// Flush writes all buffered log messages to stdout
func (bl *BufferedLogger) Flush() {
	bl.mutex.Lock()
	defer bl.mutex.Unlock()

	bl.flushLocked()
}

// flushLocked writes buffer to stdout (must be called with mutex held)
func (bl *BufferedLogger) flushLocked() {
	if bl.buffer.Len() > 0 {
		fmt.Print(bl.buffer.String())
		bl.buffer.Reset()
	}
}

// periodicFlush runs in a goroutine to periodically flush the log buffer
func (bl *BufferedLogger) periodicFlush() {
	ticker := time.NewTicker(bl.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			bl.Flush()
		case <-bl.stopChan:
			return
		}
	}
}
