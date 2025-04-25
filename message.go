package main

import "time"

// Message defines the structure for the JSON messages exchanged between client and server
type Message struct {
	// Time when the message was created
	Timestamp time.Time `json:"timestamp"`

	// Message content
	Content string `json:"content"`

	// Unique message identifier (used by client)
	MessageID string `json:"message_id,omitempty"`
}
