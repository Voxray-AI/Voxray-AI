// Package httpclient provides a shared HTTP transport and client factory for provider API calls.
// Using a tuned transport improves connection reuse under concurrent sessions and reduces latency.
package httpclient

import (
	"net"
	"net/http"
	"time"
)

const (
	defaultMaxIdleConns        = 100
	defaultMaxIdleConnsPerHost = 10
	defaultIdleConnTimeout     = 90 * time.Second
)

// DefaultTransport is a shared transport tuned for provider API calls (STT, LLM, TTS).
// - MaxIdleConnsPerHost allows connection reuse across concurrent sessions.
// - DisableCompression avoids unnecessary CPU for streaming/audio endpoints.
// - IdleConnTimeout keeps connections warm without holding them indefinitely.
var DefaultTransport = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	DialContext: (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext,
	MaxIdleConns:          defaultMaxIdleConns,
	MaxIdleConnsPerHost:   defaultMaxIdleConnsPerHost,
	IdleConnTimeout:       defaultIdleConnTimeout,
	TLSHandshakeTimeout:   10 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
	DisableCompression:    true, // streaming/audio endpoints typically don't benefit
	ResponseHeaderTimeout: 30 * time.Second,
}

// Client returns an http.Client that uses DefaultTransport with the given timeout.
// Use this for provider services (Sarvam, Anthropic, ElevenLabs, etc.) so connection
// reuse is shared across concurrent sessions.
func Client(timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: DefaultTransport,
		Timeout:   timeout,
	}
}
