package unfurl

import (
	"context"
	"errors"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"
)

const (
	DefaultDelayMs             = 150
	DefaultMaxBodySize         = 4096
	DefaultMaxRedirects        = 10
	DefaultUserAgent           = "RedirectFollower/1.0 (contact: admin@example.com)"
	DefaultRequestTimeout      = 10 * time.Second
	DefaultMaxIdleConns        = 10
	DefaultIdleConnTimeout     = 30 * time.Second
	DefaultTLSHandshakeTimeout = 5 * time.Second
)

// RedirectResponse represents a single step in a redirect chain.
type RedirectResponse struct {
	URL       string            `json:"url"`
	TargetURL string            `json:"target_url"`
	Headers   map[string]string `json:"headers"`
	Body      string            `json:"body"`
}

// Requester configures and executes HTTP requests for redirect following.
type Requester struct {
	DelayMs             int
	MaxBodySize         int
	MaxRedirects        int
	UserAgent           string
	RequestTimeout      time.Duration
	MaxIdleConns        int
	IdleConnTimeout     time.Duration
	TLSHandshakeTimeout time.Duration
	client              *http.Client
}

var metaRedirectRe = regexp.MustCompile(`(?s)<meta\s+http-equiv=["']refresh["']\s+content=["']\d+;\s*url=([^"']+)["'].*?>`)

// NewRequester creates a Requester with default values applied where unspecified.
func NewRequester(r Requester) *Requester {
	if r.DelayMs == 0 {
		r.DelayMs = DefaultDelayMs
	}
	if r.MaxBodySize == 0 {
		r.MaxBodySize = DefaultMaxBodySize
	}
	if r.MaxRedirects == 0 {
		r.MaxRedirects = DefaultMaxRedirects
	}
	if r.UserAgent == "" {
		r.UserAgent = DefaultUserAgent
	}
	if r.RequestTimeout == 0 {
		r.RequestTimeout = DefaultRequestTimeout
	}
	if r.MaxIdleConns == 0 {
		r.MaxIdleConns = DefaultMaxIdleConns
	}
	if r.IdleConnTimeout == 0 {
		r.IdleConnTimeout = DefaultIdleConnTimeout
	}
	if r.TLSHandshakeTimeout == 0 {
		r.TLSHandshakeTimeout = DefaultTLSHandshakeTimeout
	}

	r.client = &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: &http.Transport{
			MaxIdleConns:        r.MaxIdleConns,
			IdleConnTimeout:     r.IdleConnTimeout,
			DisableCompression:  true,
			TLSHandshakeTimeout: r.TLSHandshakeTimeout,
		},
	}

	return &r
}

// request executes a secure HTTP GET request to the given URL.
func (r *Requester) request(url string) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.RequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", r.UserAgent)
	req.Header.Set("Accept", "text/html,text/plain;q=0.9")
	req.Header.Set("Connection", "close")

	return r.client.Do(req)
}

// FollowRedirects traces URL redirects up to a limit, returning the response chain.
func (r *Requester) FollowRedirects(url string) ([]RedirectResponse, error) {
	var responses []RedirectResponse
	targetURL := url

	if r.MaxRedirects < 1 {
		return nil, errors.New("MaxRedirects must be at least 1")
	}

	for i := range r.MaxRedirects {
		if i > 0 {
			if r.DelayMs > 0 {
				time.Sleep(time.Duration(r.DelayMs) * time.Millisecond)
			}
		}

		resp, err := r.request(targetURL)
		if err != nil {
			return responses, err
		}
		defer resp.Body.Close()

		headers := make(map[string]string)
		for key, values := range resp.Header {
			if len(values) > 0 {
				headers[key] = values[len(values)-1]
			}
		}

		var body string
		contentType := resp.Header.Get("Content-Type")
		if strings.HasPrefix(contentType, "text/plain") || strings.HasPrefix(contentType, "text/html") {
			bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, int64(r.MaxBodySize)))
			if err == nil {
				body = string(bodyBytes)
			}
		}

		entry := RedirectResponse{
			URL:       targetURL,
			TargetURL: "",
			Headers:   headers,
			Body:      body,
		}

		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			if location := resp.Header.Get("Location"); location != "" {
				targetURL = location
				entry.TargetURL = targetURL
				responses = append(responses, entry)
				continue
			}
		}

		if strings.HasPrefix(contentType, "text/html") && len(body) > 0 {
			if matches := metaRedirectRe.FindStringSubmatch(body); len(matches) > 1 {
				targetURL = matches[1]
				entry.TargetURL = targetURL
				responses = append(responses, entry)
				continue
			}
		}

		responses = append(responses, entry)
		break
	}

	slices.Reverse(responses)

	return responses, nil
}
