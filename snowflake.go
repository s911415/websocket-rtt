package main

import (
	"fmt"
	"sync"
	"time"
)

// Snowflake structure for ID generation
// Based on Twitter's Snowflake algorithm but modified for nanosecond precision
// ID = timestamp + nodeID + sequence
type Snowflake struct {
	mu            sync.Mutex
	lastTimestamp int64
	nodeID        int64
	sequence      int64

	// Constants for bit manipulation
	nodeBits       int64
	sequenceBits   int64
	nodeShift      int64
	timestampShift int64
	sequenceMask   int64

	// Epoch start time (custom epoch)
	epoch int64
}

// NewSnowflake creates and returns a new Snowflake ID generator
func NewSnowflake(nodeID int64) (*Snowflake, error) {
	const (
		nodeBits     = 10
		sequenceBits = 12
	)

	// Validate node ID is within range
	maxNodeID := int64((1 << nodeBits) - 1)
	if nodeID < 0 || nodeID > maxNodeID {
		return nil, fmt.Errorf("nodeID must be between 0 and %d", maxNodeID)
	}

	// January 1, 2020 Midnight UTC - in nanoseconds
	epoch := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).UnixNano()

	return &Snowflake{
		nodeID:         nodeID,
		lastTimestamp:  -1,
		epoch:          epoch,
		nodeBits:       nodeBits,
		sequenceBits:   sequenceBits,
		nodeShift:      sequenceBits,
		timestampShift: nodeBits + sequenceBits,
		sequenceMask:   int64((1 << sequenceBits) - 1),
	}, nil
}

// NextID generates and returns the next Snowflake ID
func (sf *Snowflake) NextID() (int64, error) {
	sf.mu.Lock()
	defer sf.mu.Unlock()

	// Get current timestamp in nanoseconds
	timestamp := time.Now().UnixNano()
	timestamp = timestamp - sf.epoch

	// Check for clock drift (if the current time is earlier than the last timestamp)
	if timestamp < sf.lastTimestamp {
		return 0, fmt.Errorf("clock moved backwards, refusing to generate id for %d nanoseconds",
			sf.lastTimestamp-timestamp)
	}

	// If we're still in the same nanosecond as the last ID, increment the sequence
	if timestamp == sf.lastTimestamp {
		sf.sequence = (sf.sequence + 1) & sf.sequenceMask
		// If the sequence overflows, wait until next nanosecond
		if sf.sequence == 0 {
			for timestamp <= sf.lastTimestamp {
				timestamp = time.Now().UnixNano() - sf.epoch
			}
		}
	} else {
		// If this is a new nanosecond, reset the sequence
		sf.sequence = 0
	}

	sf.lastTimestamp = timestamp

	// Combine the components into a single 64-bit ID
	id := (timestamp << sf.timestampShift) | (sf.nodeID << sf.nodeShift) | sf.sequence

	return id, nil
}
