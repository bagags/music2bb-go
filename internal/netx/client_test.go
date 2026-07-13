package netx

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestClientRetriesIdempotentTransientResponses(t *testing.T) {
	var calls atomic.Int32
	transport := roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
		if calls.Add(1) < 3 {
			return response(http.StatusServiceUnavailable, "retry"), nil
		}
		return response(http.StatusOK, "ok"), nil
	})

	client := New(time.Second, 2, nil)
	client.HTTP.Transport = transport
	client.Sleep = func(context.Context, time.Duration) error { return nil }
	client.Jitter = nil
	req, _ := http.NewRequest(http.MethodGet, "https://example.invalid", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if calls.Load() != 3 {
		t.Fatalf("calls = %d, want 3", calls.Load())
	}
}

func TestClientDoesNotRetryWrites(t *testing.T) {
	var calls atomic.Int32
	transport := roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
		calls.Add(1)
		return response(http.StatusServiceUnavailable, "ambiguous"), nil
	})

	client := New(time.Second, 2, nil)
	client.HTTP.Transport = transport
	req, _ := http.NewRequest(http.MethodPost, "https://example.invalid", strings.NewReader("x"))
	if _, err := client.Do(req); err == nil {
		t.Fatal("expected error")
	}
	if calls.Load() != 1 {
		t.Fatalf("calls = %d, want 1", calls.Load())
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}
}

func TestLimiterHonorsCancellation(t *testing.T) {
	limiter := NewTokenLimiter(1, 1)
	if err := limiter.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := limiter.Wait(ctx); err != context.Canceled {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}
