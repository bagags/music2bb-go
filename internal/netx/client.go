package netx

import (
	"context"
	"errors"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Sleeper makes retry timing deterministic in tests.
type Sleeper func(context.Context, time.Duration) error

// Limiter is the small interface required by Client.
type Limiter interface {
	Wait(context.Context) error
}

// Client wraps one reusable HTTP client with rate limiting and safe retries.
// Requests are retried only when the method is idempotent and the body can be
// recreated by net/http.
type Client struct {
	HTTP        *http.Client
	Limiter     Limiter
	MaxAttempts int
	BaseBackoff time.Duration
	MaxBackoff  time.Duration
	Sleep       Sleeper
	Jitter      func(time.Duration) time.Duration
}

func New(timeout time.Duration, maxConns int, limiter Limiter) *Client {
	if maxConns < 1 {
		maxConns = 8
	}
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          maxConns * 2,
		MaxIdleConnsPerHost:   maxConns,
		MaxConnsPerHost:       maxConns,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: timeout,
	}
	return &Client{
		HTTP:        &http.Client{Transport: transport, Timeout: timeout},
		Limiter:     limiter,
		MaxAttempts: 3,
		BaseBackoff: 250 * time.Millisecond,
		MaxBackoff:  3 * time.Second,
		Sleep:       sleepContext,
		Jitter: func(d time.Duration) time.Duration {
			if d <= 1 {
				return 0
			}
			return time.Duration(rand.Int64N(int64(d / 2)))
		},
	}
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	if c.HTTP == nil {
		return nil, errors.New("netx: nil HTTP client")
	}
	attempts := c.MaxAttempts
	if attempts < 1 {
		attempts = 1
	}
	canRetry := isIdempotent(req.Method) && (req.Body == nil || req.GetBody != nil)
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if c.Limiter != nil {
			if err := c.Limiter.Wait(req.Context()); err != nil {
				return nil, err
			}
		}

		current := req
		if attempt > 0 {
			current = req.Clone(req.Context())
			if req.GetBody != nil {
				body, err := req.GetBody()
				if err != nil {
					return nil, err
				}
				current.Body = body
			}
		}

		resp, err := c.HTTP.Do(current)
		if err == nil && !retryableStatus(resp.StatusCode) {
			return resp, nil
		}
		if err == nil {
			lastErr = errors.New(resp.Status)
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		} else {
			lastErr = err
			if !retryableError(err) {
				return nil, err
			}
		}
		if !canRetry || attempt == attempts-1 {
			if err != nil {
				return nil, err
			}
			return nil, lastErr
		}

		delay := c.backoff(attempt)
		if resp != nil && resp.StatusCode == http.StatusTooManyRequests {
			if retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"), time.Now()); retryAfter > delay {
				delay = retryAfter
			}
		}
		if c.Jitter != nil {
			delay += c.Jitter(delay)
		}
		sleep := c.Sleep
		if sleep == nil {
			sleep = sleepContext
		}
		if err := sleep(req.Context(), delay); err != nil {
			return nil, err
		}
	}
	return nil, lastErr
}

func (c *Client) CloseIdleConnections() {
	if c != nil && c.HTTP != nil {
		c.HTTP.CloseIdleConnections()
	}
}

func (c *Client) backoff(attempt int) time.Duration {
	base := c.BaseBackoff
	if base <= 0 {
		base = 250 * time.Millisecond
	}
	maximum := c.MaxBackoff
	if maximum <= 0 {
		maximum = 3 * time.Second
	}
	delay := base << attempt
	if delay > maximum {
		return maximum
	}
	return delay
}

func isIdempotent(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodPut, http.MethodDelete:
		return true
	default:
		return false
	}
}

func retryableStatus(status int) bool {
	return status == http.StatusRequestTimeout || status == http.StatusTooManyRequests || status >= 500
}

func retryableError(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var netErr net.Error
	return errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary())
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(value); err == nil && when.After(now) {
		return when.Sub(now)
	}
	return 0
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// TokenLimiter is a cancellation-aware token bucket shared by all workers.
type TokenLimiter struct {
	interval time.Duration
	burst    int
	mu       sync.Mutex
	next     time.Time
	credit   int
	now      func() time.Time
	sleep    Sleeper
}

func NewTokenLimiter(requestsPerSecond float64, burst int) *TokenLimiter {
	if requestsPerSecond <= 0 {
		requestsPerSecond = 4
	}
	if burst < 1 {
		burst = 1
	}
	return &TokenLimiter{
		interval: time.Duration(float64(time.Second) / requestsPerSecond),
		burst:    burst,
		credit:   burst,
		now:      time.Now,
		sleep:    sleepContext,
	}
}

func (l *TokenLimiter) Wait(ctx context.Context) error {
	if l == nil || l.interval <= 0 {
		return nil
	}
	l.mu.Lock()
	now := l.now()
	if l.next.IsZero() {
		l.next = now.Add(l.interval)
	}
	if l.credit > 0 {
		l.credit--
		l.mu.Unlock()
		return nil
	}
	if !now.Before(l.next) {
		l.next = now.Add(l.interval)
		l.mu.Unlock()
		return nil
	}
	wait := l.next.Sub(now)
	l.next = l.next.Add(l.interval)
	l.mu.Unlock()
	return l.sleep(ctx, wait)
}
