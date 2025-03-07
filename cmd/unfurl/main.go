package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/bnnanet/unfurl"
)

const (
	name         = "unfurl"
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

	delayMs := 10
	flag.IntVar(&delayMs, "delay-ms", delayMs, "Delay between requests in milliseconds")

	maxBodySize := unfurl.DefaultMaxBodySize
	flag.IntVar(&maxBodySize, "max-body-size", maxBodySize, "Maximum body size in bytes")

	maxRedirects := unfurl.DefaultMaxRedirects
	flag.IntVar(&maxRedirects, "max-redirects", maxRedirects, "Maximum number of redirects")

	depth := 0
	flag.IntVar(&depth, "depth", depth, "Gives back the result at depth N")

	var showJSON bool
	flag.BoolVar(&showJSON, "json", showJSON, "Show full results as JSON")

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

	flag.Usage = func() {
		printVersion()
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "USAGE\n")
		fmt.Fprintf(os.Stderr, "   unfurl [options] <url>\n")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "EXAMPLES\n")
		fmt.Fprintf(os.Stderr, "   unfurl --depth 1 'https://tinyurl.com/2u7bcuny'\n")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "OPTIONS\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	// Handle --version flag after parsing
	if showVersion {
		printVersion()
		return
	}

	// Check for URL argument
	if flag.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "Error: exactly one URL argument is required\n")
		flag.Usage()
		os.Exit(1)
	}
	url := flag.Arg(0)

	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		fmt.Fprintf(os.Stderr, "Error: URL must start with http:// or https://\n")
		os.Exit(1)
	}

	effectiveMaxRedirects := maxRedirects
	if depth != 0 {
		if depth < maxRedirects {
			effectiveMaxRedirects = depth
		}
	}

	// Construct default User-Agent with version info
	defaultUA := fmt.Sprintf("unfurl/%s", version)
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
		MaxRedirects:        effectiveMaxRedirects,
		UserAgent:           userAgent,
		RequestTimeout:      requestTimeout,
		MaxIdleConns:        maxIdleConns,
		IdleConnTimeout:     idleConnTimeout,
		TLSHandshakeTimeout: tlsHandshakeTimeout,
	})
	if r.MaxRedirects < 1 {
		fmt.Fprintf(os.Stderr, "MaxRedirects must be at least 1")
		os.Exit(1)
	}

	results, err := r.FollowRedirects(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error following redirects: %v\n", err)
		os.Exit(1)
	}

	if depth > 0 {
		actualDepth := len(results)
		if len(results[0].TargetURL) == 0 {
			actualDepth -= 1
		}
		redirect := results[0]

		if showJSON {
			_ = json.NewEncoder(os.Stdout).Encode(redirect)
		} else {
			fmt.Println(redirect.TargetURL)
		}

		if depth > actualDepth {
			fmt.Fprintf(os.Stderr, "Error: --depth is %d, but redirect count is %d\n", depth, actualDepth)
			os.Exit(1)
		}
		return
	}

	if showJSON {

		_ = json.NewEncoder(os.Stdout).Encode(results)
		return
	}
	for _, redirect := range results {
		if len(redirect.TargetURL) > 0 {
			fmt.Println(redirect.TargetURL)
		}
	}
	fmt.Println(url)

	// Check if we hit the redirect limit
	if len(results) == maxRedirects && len(results[0].TargetURL) != 0 {
		if depth == 0 {
			fmt.Fprintf(os.Stderr, "Error: too many redirects (limit %d)\n", effectiveMaxRedirects)
			os.Exit(1)
		}
	}
}
