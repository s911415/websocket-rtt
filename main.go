package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
)

func parseHeaderArguments(headers *headerFlags) map[string]string {
	// Process headers and add default User-Agent if not specified
	headersMap := make(map[string]string)

	// Add default User-Agent header if not specified by user
	hasUserAgent := false
	for name := range headers.headers {
		if strings.ToLower(name) == "user-agent" {
			hasUserAgent = true
			break
		}
	}

	if !hasUserAgent {
		headersMap["User-Agent"] = GetUserAgent()
	}

	// Add user-specified headers
	for name, value := range headers.headers {
		headersMap[name] = value
	}

	return headersMap
}

func main() {
	// Define command-line flags to determine mode and address
	mode := flag.String("mode", "", "Operation mode: 'server' or 'client'")
	addr := flag.String("addr", "localhost:8080", "WebSocket server address")
	interval := flag.Uint64("interval", 100, "Interval of messages in miliseconds")
	useTLS := flag.Bool("tls", false, "Use TLS for secure connection")
	insecureSkipVerify := flag.Bool("k", false, "Skip TLS certificate verification (insecure)")
	keylogFile := flag.String("keylogger", "", "Path to TLS key log file (overrides SSLKEYLOGFILE env var)")
	showVersion := flag.Bool("version", false, "Show version information and exit")

	// Define a custom flag for headers that can be specified multiple times
	var headers headerFlags
	flag.Var(&headers, "H", "Add HTTP request header (can be specified multiple times, e.g., -H 'Authorization: Bearer xyz')")

	// Define custom usage to provide clear instructions
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "WebSocket Server/Client Application\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s -mode [server|client] -addr [host:port] [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		fmt.Fprintf(os.Stderr, "  -mode string\n")
		fmt.Fprintf(os.Stderr, "        Operation mode: 'server' or 'client' (required)\n")
		fmt.Fprintf(os.Stderr, "  -addr string\n")
		fmt.Fprintf(os.Stderr, "        WebSocket server address (default \"localhost:8080\")\n")
		fmt.Fprintf(os.Stderr, "  -interval number\n")
		fmt.Fprintf(os.Stderr, "        Interval for each message, unit: ms (default \"100 ms\")\n")
		fmt.Fprintf(os.Stderr, "  -tls\n")
		fmt.Fprintf(os.Stderr, "        Use TLS for secure connection\n")
		fmt.Fprintf(os.Stderr, "  -k\n")
		fmt.Fprintf(os.Stderr, "        Skip TLS certificate verification (insecure)\n")
		fmt.Fprintf(os.Stderr, "  -keylogger string\n")
		fmt.Fprintf(os.Stderr, "        Path to TLS key log file (overrides SSLKEYLOGFILE env var)\n")
		fmt.Fprintf(os.Stderr, "  -H string\n")
		fmt.Fprintf(os.Stderr, "        Add HTTP request header (can be specified multiple times, e.g., -H 'Authorization: Bearer xyz')\n")
		fmt.Fprintf(os.Stderr, "  -version\n")
		fmt.Fprintf(os.Stderr, "        Show version information and exit\n\n")
		fmt.Fprintf(os.Stderr, "Environment variables:\n")
		fmt.Fprintf(os.Stderr, "  SSLKEYLOGFILE\n")
		fmt.Fprintf(os.Stderr, "        Path to file for storing SSL key log information\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  Start server:  %s -mode server -addr :8080\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  Start client:  %s -mode client -addr localhost:8080\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  Start TLS client:  %s -mode client -addr localhost:8443 -tls\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  Start client with custom headers: %s -mode client -H 'Authorization: Bearer xyz' -H 'X-Custom: Value'\n", os.Args[0])
	}

	flag.Parse()

	// Show version information if requested
	if *showVersion {
		fmt.Println(GetVersionInfo())
		return
	}

	// Validate mode parameter
	if *mode == "" {
		fmt.Fprintln(os.Stderr, "Error: -mode parameter is required")
		flag.Usage()
		os.Exit(1)
	}

	// Determine which key log file path to use
	var keyLogFilePath string
	if *keylogFile != "" {
		// Prefer the path specified by -keylogger parameter
		keyLogFilePath = *keylogFile
	} else {
		// If -keylogger not specified, try the environment variable
		keyLogFilePath = os.Getenv("SSLKEYLOGFILE")
	}

	// Create config structure to pass parameters
	config := Config{
		Addr:               *addr,
		Interval:           *interval,
		UseTLS:             *useTLS,
		InsecureSkipVerify: *insecureSkipVerify,
		SSLKeyLogFile:      keyLogFilePath,
		Headers:            parseHeaderArguments(&headers),
	}

	// Handle different modes
	switch *mode {
	case "server":
		// Run in server mode
		fmt.Println("Starting WebSocket server on", *addr)
		if err := startServer(config); err != nil {
			log.Fatal("Server error:", err)
		}
	case "client":
		// Run in client mode
		fmt.Println("Starting WebSocket client connecting to", *addr)
		if *useTLS {
			fmt.Println("TLS enabled")
			if *insecureSkipVerify {
				fmt.Println("Warning: TLS certificate verification disabled")
			}
			if config.SSLKeyLogFile != "" {
				fmt.Printf("TLS keys will be logged to: %s\n", config.SSLKeyLogFile)
			}
		}
		if err := startClient(config); err != nil {
			log.Fatal("Client error:", err)
		}
	default:
		fmt.Fprintf(os.Stderr, "Error: Invalid mode '%s'. Must be either 'server' or 'client'\n", *mode)
		flag.Usage()
		os.Exit(1)
	}
}
