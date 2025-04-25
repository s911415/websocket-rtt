package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// generateRandomString creates a random string of specified length
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// setupSSLKeyLogger configures TLS key logging if SSLKEYLOGFILE is set
func setupSSLKeyLogger(keyLogFile string) (func(string), error) {
	if keyLogFile == "" {
		return nil, nil
	}

	// Open the key log file for appending
	f, err := os.OpenFile(keyLogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open SSL key log file: %v", err)
	}

	// Return a keylog callback function that appends to the file
	return func(s string) {
		f.WriteString(s + "\n")
	}, nil
}

func startClient(config Config) error {
	logger := NewBufferedLogger(4096, 250*time.Millisecond)
	defer logger.Stop()

	// Initialize random number generator
	rand.Seed(time.Now().UnixNano())

	// Create a Snowflake ID generator
	// Using a random node ID between 0-1023
	nodeID := rand.Int63n(1024)
	snowflake, err := NewSnowflake(nodeID)
	if err != nil {
		return fmt.Errorf("failed to create snowflake generator: %v", err)
	}

	logger.Write(fmt.Sprintf("Initialized Snowflake ID generator with node ID: %d", nodeID))

	// Configure WebSocket dialer with TLS if enabled
	dialer := websocket.DefaultDialer
	if config.UseTLS {
		// Set up TLS configuration
		tlsConfig := &tls.Config{
			InsecureSkipVerify: config.InsecureSkipVerify,
		}

		// Configure SSL key logging if requested
		if config.SSLKeyLogFile != "" {
			keyLogger, err := setupSSLKeyLogger(config.SSLKeyLogFile)
			if err != nil {
				return err
			}
			if keyLogger != nil {
				tlsConfig.KeyLogWriter = &KeyLogWriter{keyLogger: keyLogger}
				logger.Write(fmt.Sprintf("TLS key logging enabled to: %s", config.SSLKeyLogFile))
			}
		}

		dialer.TLSClientConfig = tlsConfig
	}

	// Construct WebSocket URL with appropriate scheme
	scheme := "ws"
	if config.UseTLS {
		scheme = "wss"
	}
	url := fmt.Sprintf("%s://%s", scheme, config.Addr)

	// Prepare request headers
	header := http.Header{}
	for name, value := range config.Headers {
		header.Set(name, value)
	}

	// Connect to the WebSocket server
	conn, _, err := dialer.Dial(url, header)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %v", err)
	}
	defer conn.Close()

	logger.Write(fmt.Sprintf("Connected to WebSocket server at %s", url))

	// Channel to signal when to exit
	done := make(chan struct{})
	// Channel to coordinate message sending after receiving response
	sendNext := make(chan struct{}, 1)
	// Channel for signal handling
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt) // Catch SIGINT (Ctrl+C)

	// Statistics for RTT measurements
	var stats struct {
		sync.Mutex
		messageCount int
		totalRTT     time.Duration
		minRTT       time.Duration
		maxRTT       time.Duration
	}
	stats.minRTT = time.Duration(1<<63 - 1) // Initialize to max possible duration
	stats.maxRTT = time.Duration(-1 << 63)  // Initialize to min possible duration

	// Set up a goroutine to read messages from the server
	go func() {
		defer close(done)
		for {
			messageType, serverMessage, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					return
				}
				logger.Write(fmt.Sprintf("Error reading message: %v", err))
				return
			}

			switch messageType {
			case websocket.TextMessage:
				var msg Message
				if err := json.Unmarshal(serverMessage, &msg); err != nil {
					logger.Write(fmt.Sprintf("Error parsing message: %v", err))
					continue
				}

				// Calculate round-trip time with nanosecond precision
				now := time.Now().UTC()
				rtt := now.Sub(msg.Timestamp)

				// Update statistics
				stats.Lock()
				stats.messageCount++
				stats.totalRTT += rtt
				if rtt < stats.minRTT {
					stats.minRTT = rtt
				}
				if rtt > stats.maxRTT {
					stats.maxRTT = rtt
				}
				stats.Unlock()

				// log.Printf("Received: %s (ID: %s)", msg.Content, msg.MessageID)
				logger.Write(fmt.Sprintf("Round-trip time: %d us", rtt.Microseconds()))

				// Signal to send the next message
				sendNext <- struct{}{}
			case websocket.PingMessage:
				// Received a ping from the server, send back a pong
				err := conn.WriteMessage(websocket.PongMessage, nil)
				if err != nil {
					logger.Write(fmt.Sprintf("Error sending pong response: %v", err))
					return // Exit the read loop if pong fails
				}
				logger.Write("Received ping, sent pong.")
			}
		}
	}()

	// Send the first message to start the cycle
	sendNext <- struct{}{}

	// Main loop for sending messages
	messagesSent := 0
	for {
		select {
		case <-sendNext:
			// Generate a message ID using Snowflake algorithm
			snowflakeID, err := snowflake.NextID()
			if err != nil {
				log.Printf("Error generating snowflake ID: %v", err)
				continue
			}
			messageID := fmt.Sprintf("%d", snowflakeID)

			// Generate random content
			content := generateRandomString(16)

			// Create message with current timestamp
			msg := Message{
				Timestamp: time.Now().UTC(),
				Content:   content,
				MessageID: messageID,
			}

			// Marshal the message to JSON
			msgJSON, err := json.Marshal(msg)
			if err != nil {
				log.Printf("Error marshaling message: %v", err)
				continue
			}

			// Send the message
			if err := conn.WriteMessage(websocket.TextMessage, msgJSON); err != nil {
				log.Printf("Error sending message: %v", err)
				return err
			}

			// log.Printf("Sent message: %s (ID: %s)", content, messageID)

			messagesSent++

			// Small delay to prevent flooding the connection
			time.Sleep(time.Duration(config.Interval) * time.Millisecond)

		case <-done:
			return nil

		case <-interrupt:
			// When SIGINT is received, display stats before exiting

			// Ensure we flush the log buffer
			logger.Flush()

			stats.Lock()
			defer stats.Unlock()

			if stats.messageCount == 0 {
				fmt.Println("\nNo messages were exchanged. Exiting...")
				return nil
			}

			// Calculate average
			avgRTT := stats.totalRTT / time.Duration(stats.messageCount)

			// Display ping-style statistics
			fmt.Printf("\nApproximate round trip times in micro-seconds:\n")
			fmt.Printf("    Minimum = %dus, Maximum = %dus, Average = %dus\n",
				stats.minRTT.Microseconds(),
				stats.maxRTT.Microseconds(),
				avgRTT.Microseconds())
			fmt.Printf("Messages sent: %d\n", messagesSent)

			// Send close message to server (optional but polite)
			err := conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				return fmt.Errorf("write close: %v", err)
			}

			// Wait for the server to close the connection
			select {
			case <-done:
				// Connection closed by server
			case <-time.After(time.Second):
				// Timed out waiting for server to close
			}

			return nil
		}
	}
}

// KeyLogWriter is a wrapper to implement io.Writer for TLS key logging
type KeyLogWriter struct {
	keyLogger func(string)
}

// Write implements io.Writer for KeyLogWriter
func (klw *KeyLogWriter) Write(p []byte) (n int, err error) {
	klw.keyLogger(string(p))
	return len(p), nil
}
