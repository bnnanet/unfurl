package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/bnnanet/unfurl"
)

const (
	name         = "unfurld"
	licenseYear  = "2025"
	licenseOwner = "AJ ONeal"
	licenseType  = "MPL-2.0"
)

// set by GoReleaser via ldflags
var (
	version = ""
	commit  = ""
	date    = ""
)

// workaround for `tinygo` ldflag replacement handling not allowing default values
// See <https://github.com/tinygo-org/tinygo/issues/2976>
func init() {
	if len(version) == 0 {
		version = "0.0.0-dev"
	}
	if len(date) == 0 {
		date = "0001-01-01T00:00:00Z"
	}
	if len(commit) == 0 {
		commit = "0000000"
	}
}

// printVersion displays the version, commit, and build date.
func printVersion() {
	fmt.Fprintf(os.Stderr, "%s v%s %s (%s)\n", name, version, commit[:7], date)
	fmt.Fprintf(os.Stderr, "Copyright (C) %s %s\n", licenseYear, licenseOwner)
	fmt.Fprintf(os.Stderr, "Licensed under the %s license\n", licenseType)
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "-V", "version", "--version":
			printVersion()
			return
		}
	}

	var showVersion bool
	flag.BoolVar(&showVersion, "version", false, "Print version and exit")

	delayMs := unfurl.DefaultDelayMs
	flag.IntVar(&delayMs, "delay-ms", delayMs, "Delay between requests in milliseconds")

	maxBodySize := unfurl.DefaultMaxBodySize
	flag.IntVar(&maxBodySize, "max-body-size", maxBodySize, "Maximum body size in bytes")

	maxRedirects := unfurl.DefaultMaxRedirects
	flag.IntVar(&maxRedirects, "max-redirects", maxRedirects, "Maximum number of redirects")

	var userAgent string
	flag.StringVar(&userAgent, "user-agent", userAgent, "Custom User-Agent string (default includes version and build info)")

	requestTimeout := unfurl.DefaultRequestTimeout
	flag.DurationVar(&requestTimeout, "request-timeout", requestTimeout, "Timeout for each request")

	maxIdleConns := unfurl.DefaultMaxIdleConns
	flag.IntVar(&maxIdleConns, "max-idle-conns", maxIdleConns, "Maximum number of idle connections")

	idleConnTimeout := unfurl.DefaultIdleConnTimeout
	flag.DurationVar(&idleConnTimeout, "idle-conn-timeout", idleConnTimeout, "Timeout for idle connections")

	tlsHandshakeTimeout := unfurl.DefaultTLSHandshakeTimeout
	flag.DurationVar(&tlsHandshakeTimeout, "tls-handshake-timeout", tlsHandshakeTimeout, "Timeout for TLS handshake")

	port := 8080
	flag.IntVar(&port, "port", port, "Port to listen on")

	bind := "127.0.0.1"
	flag.StringVar(&bind, "bind", bind, "Address to bind to")

	flag.Usage = func() {
		printVersion()
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "USAGE\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	// Handle --version flag after parsing
	if showVersion {
		printVersion()
		return
	}

	// Check PORT environment variable, override default if set
	if envPort := os.Getenv("PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil && p > 0 {
			port = p
		} else {
			log.Printf("Invalid PORT environment variable value: %s, using default or flag value", envPort)
		}
	}

	// Check BIND environment variable, override default if set
	if envBind := os.Getenv("BIND"); envBind != "" {
		bind = envBind
	}

	// Construct default User-Agent with version info
	defaultUA := fmt.Sprintf("RedirectFollower/%s", version)
	if commit != "0000000" {
		defaultUA += fmt.Sprintf(" (%s)", commit)
	}
	if userAgent == "" {
		userAgent = defaultUA
	}

	// Create Requester with flag values
	r := unfurl.NewRequester(unfurl.Requester{
		DelayMs:             delayMs,
		MaxBodySize:         maxBodySize,
		MaxRedirects:        maxRedirects,
		UserAgent:           userAgent,
		RequestTimeout:      requestTimeout,
		MaxIdleConns:        maxIdleConns,
		IdleConnTimeout:     idleConnTimeout,
		TLSHandshakeTimeout: tlsHandshakeTimeout,
	})
	if r.MaxRedirects < 1 {
		log.Fatalf("MaxRedirects must be at least 1")
	}

	http.HandleFunc("GET /api/unfurl", newUnfurlHandler(r))

	// Use the bind address and port
	addr := fmt.Sprintf("%s:%d", bind, port)

	// Print configuration as aligned key-value pairs
	fmt.Printf("\n")
	fmt.Printf("Server with configuration:\n")
	fmt.Printf("\n")
	fmt.Printf("   %-20s %dms\n", "Delay:", r.DelayMs)
	fmt.Printf("   %-20s %d bytes\n", "Max Body Size:", r.MaxBodySize)
	fmt.Printf("   %-20s %d\n", "Max Redirects:", r.MaxRedirects)
	fmt.Printf("   %-20s %s\n", "User-Agent:", r.UserAgent)
	fmt.Printf("   %-20s %s\n", "Request Timeout:", r.RequestTimeout)
	fmt.Printf("   %-20s %d\n", "Max Idle Conns:", r.MaxIdleConns)
	fmt.Printf("   %-20s %s\n", "Idle Conn Timeout:", r.IdleConnTimeout)
	fmt.Printf("   %-20s %s\n", "TLS Handshake Timeout:", r.TLSHandshakeTimeout)
	fmt.Printf("   %-20s %s\n", "Bind Address:", bind)
	fmt.Printf("   %-20s %d\n", "Port:", port)
	fmt.Printf("\n")
	fmt.Printf("Example usage:\n")
	fmt.Printf("\n")
	fmt.Printf("   curl 'http://%s/api/unfurl?url=https://example.com' |\n", addr)
	fmt.Printf("      jq -r .result[0].url\n")
	fmt.Printf("\n")

	log.Printf("Listening on %s ...\n", addr)

	log.Fatal(http.ListenAndServe(addr, nil))
}

// apiResponse defines the structure for all API responses.
type apiResponse struct {
	Success bool                      `json:"success"`
	Result  []unfurl.RedirectResponse `json:"result,omitempty"`
	Error   string                    `json:"error,omitempty"`
}

func newUnfurlHandler(r *unfurl.Requester) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		url := req.URL.Query().Get("url")
		if url == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(apiResponse{Error: "Missing url parameter"})
			return
		}

		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(apiResponse{Error: "Invalid URL scheme"})
			return
		}

		// Get optional depth parameter
		depthStr := req.URL.Query().Get("depth")
		effectiveMaxRedirects := r.MaxRedirects
		if depthStr != "" {
			if depth, err := strconv.Atoi(depthStr); err == nil && depth >= 0 {
				if depth != 0 && depth < r.MaxRedirects {
					effectiveMaxRedirects = depth
				}
			} else {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(apiResponse{Error: "Invalid depth parameter: must be a positive integer greater than 0"})
				return
			}
		}

		// Create a temporary Requester with the effective max redirects
		tempR := unfurl.NewRequester(unfurl.Requester{
			DelayMs:             r.DelayMs,
			MaxBodySize:         r.MaxBodySize,
			MaxRedirects:        effectiveMaxRedirects,
			UserAgent:           r.UserAgent,
			RequestTimeout:      r.RequestTimeout,
			MaxIdleConns:        r.MaxIdleConns,
			IdleConnTimeout:     r.IdleConnTimeout,
			TLSHandshakeTimeout: r.TLSHandshakeTimeout,
		})

		results, err := tempR.FollowRedirects(url)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(apiResponse{Error: fmt.Sprintf("Error following redirects: %v", err)})
			return
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(apiResponse{Success: true, Result: results})
	}
}
