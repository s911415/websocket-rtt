package main

import (
	"fmt"
	"strings"
)

// Config holds application configuration
type Config struct {
	Addr               string
	UseTLS             bool
	InsecureSkipVerify bool
	SSLKeyLogFile      string
	ServerName         string
	Headers            map[string]string
	Interval           uint64
}

// headerFlags is a custom flag type to handle multiple -H flags
type headerFlags struct {
	headers map[string]string
}

// String is the method to format the flag's value
func (h *headerFlags) String() string {
	pairs := []string{}
	for name, value := range h.headers {
		pairs = append(pairs, fmt.Sprintf("%s: %s", name, value))
	}
	return strings.Join(pairs, ", ")
}

// Set is the method to set the flag value
func (h *headerFlags) Set(value string) error {
	if h.headers == nil {
		h.headers = make(map[string]string)
	}

	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid header format (expected 'Name: Value'): %s", value)
	}

	name := strings.TrimSpace(parts[0])
	value = strings.TrimSpace(parts[1])

	// Headers are case-insensitive, so we normalize by storing with original case
	// but checking in a case-insensitive way
	h.headers[name] = value
	return nil
}
