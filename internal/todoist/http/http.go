package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const DefaultBaseURL = "https://api.todoist.com"

type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
	Verbose bool
	Logger  *log.Logger

	maxRetries int
	rng        *rand.Rand
}

type Option func(*Client)

func WithBaseURL(baseURL string) Option {
	return func(c *Client) { c.BaseURL = strings.TrimRight(baseURL, "/") }
}

func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.HTTP = h }
}

func WithVerbose(v bool) Option {
	return func(c *Client) { c.Verbose = v }
}

func WithLogger(l *log.Logger) Option {
	return func(c *Client) { c.Logger = l }
}

func New(token string, opts ...Option) *Client {
	c := &Client{
		BaseURL: DefaultBaseURL,
		Token:   token,
		HTTP: &http.Client{
			Timeout: 30 * time.Second,
		},
		Verbose:    false,
		Logger:     log.New(io.Discard, "", 0),
		maxRetries: 5,
		rng:        rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("http %d", e.StatusCode)
	}
	return fmt.Sprintf("http %d: %s", e.StatusCode, e.Body)
}

func (c *Client) DoJSON(ctx context.Context, method, path string, reqBody any, respBody any) error {
	var bodyBytes []byte
	var err error
	if reqBody != nil {
		bodyBytes, err = json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
	}
	headers := map[string]string{
		"Accept":       "application/json",
		"Content-Type": "application/json",
	}
	status, respBytes, err := c.doWithRetry(ctx, method, path, headers, bodyBytes)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return &HTTPError{StatusCode: status, Body: strings.TrimSpace(string(respBytes))}
	}
	if respBody == nil {
		return nil
	}
	if len(respBytes) == 0 || string(respBytes) == "null" {
		return nil
	}
	if err := json.Unmarshal(respBytes, respBody); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func (c *Client) DoForm(ctx context.Context, path string, values url.Values, respBody any) error {
	bodyBytes := []byte(values.Encode())
	headers := map[string]string{
		"Accept":       "application/json",
		"Content-Type": "application/x-www-form-urlencoded",
	}
	status, respBytes, err := c.doWithRetry(ctx, http.MethodPost, path, headers, bodyBytes)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return &HTTPError{StatusCode: status, Body: strings.TrimSpace(string(respBytes))}
	}
	if respBody == nil {
		return nil
	}
	if len(respBytes) == 0 || string(respBytes) == "null" {
		return nil
	}
	if err := json.Unmarshal(respBytes, respBody); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func (c *Client) doWithRetry(ctx context.Context, method, path string, headers map[string]string, body []byte) (int, []byte, error) {
	fullURL := c.BaseURL + path

	var lastStatus int
	var lastBody []byte
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		status, respBytes, retryAfter, err := c.doOnce(ctx, method, fullURL, headers, body)
		lastStatus, lastBody, lastErr = status, respBytes, err

		if err != nil {
			if attempt == c.maxRetries {
				return 0, nil, err
			}
			c.sleepBackoff(ctx, attempt, 0)
			continue
		}

		if status >= 200 && status < 300 {
			return status, respBytes, nil
		}

		switch status {
		case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout, http.StatusInternalServerError:
			if attempt == c.maxRetries {
				return status, respBytes, nil
			}
			c.sleepBackoff(ctx, attempt, retryAfter)
			continue
		default:
			return status, respBytes, nil
		}
	}

	if lastErr != nil {
		return 0, nil, lastErr
	}
	return lastStatus, lastBody, nil
}

func (c *Client) doOnce(ctx context.Context, method, fullURL string, headers map[string]string, body []byte) (int, []byte, time.Duration, error) {
	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return 0, nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("User-Agent", "htd/0.1")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	if c.Verbose {
		c.Logger.Printf("todoist request: %s %s", method, redactURL(fullURL))
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, nil, 0, err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, 0, err
	}

	retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))

	if c.Verbose {
		c.Logger.Printf("todoist response: %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	return resp.StatusCode, b, retryAfter, nil
}

func (c *Client) sleepBackoff(ctx context.Context, attempt int, retryAfter time.Duration) {
	if retryAfter > 0 {
		select {
		case <-time.After(retryAfter):
		case <-ctx.Done():
		}
		return
	}

	// Exponential: 0.5s, 1s, 2s, 4s, ... + jitter.
	base := 500 * time.Millisecond
	d := base * time.Duration(1<<attempt)
	if d > 10*time.Second {
		d = 10 * time.Second
	}
	jitter := time.Duration(c.rng.Intn(200)) * time.Millisecond
	d += jitter

	select {
	case <-time.After(d):
	case <-ctx.Done():
	}
}

func redactURL(u string) string {
	// No token in URL (we use Bearer header), but still strip query params to reduce noise.
	parsed, err := url.Parse(u)
	if err != nil {
		return u
	}
	parsed.RawQuery = ""
	return parsed.String()
}

// parseRetryAfter parses HTTP Retry-After, returning 0 if missing or invalid.
func parseRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		d := time.Until(t)
		if d < 0 {
			return 0
		}
		return d
	}
	return 0
}
