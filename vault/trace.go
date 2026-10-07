package vault

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/SomeBlackMagic/vault-manager/logging"
)

// tracingTransport logs HTTP method, redacted URL, status and duration at
// trace level. Request and response bodies and headers (which carry the
// Vault token and secret values) are never logged.
type tracingTransport struct {
	next http.RoundTripper
	log  *slog.Logger
}

func newTracingTransport(next http.RoundTripper, log *slog.Logger) http.RoundTripper {
	return &tracingTransport{next: next, log: logging.OrDiscard(log)}
}

func (t *tracingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if !t.log.Enabled(req.Context(), logging.LevelTrace) {
		return t.next.RoundTrip(req)
	}

	start := time.Now()
	res, err := t.next.RoundTrip(req)
	attrs := []any{
		"method", req.Method,
		"url", RedactURL(req.URL),
		logging.KeyDuration, time.Since(start),
	}
	if err != nil {
		t.log.Log(req.Context(), logging.LevelTrace, "vault http request failed",
			append(attrs, logging.KeyError, err.Error())...)
		return res, err
	}
	t.log.Log(req.Context(), logging.LevelTrace, "vault http request",
		append(attrs, "status", res.StatusCode)...)
	return res, nil
}

var sensitiveQueryParts = []string{
	"token", "secret", "password", "passwd", "key", "auth", "credential", "signature", "nonce",
}

// RedactURL renders u without user info and with the values of sensitive
// query parameters replaced by "REDACTED".
func RedactURL(u *url.URL) string {
	if u == nil {
		return ""
	}
	c := *u
	c.User = nil
	if c.RawQuery != "" {
		q := c.Query()
		for name := range q {
			if isSensitiveParam(name) {
				q[name] = []string{"REDACTED"}
			}
		}
		c.RawQuery = q.Encode()
	}
	return c.String()
}

func isSensitiveParam(name string) bool {
	n := strings.ToLower(name)
	for _, part := range sensitiveQueryParts {
		if strings.Contains(n, part) {
			return true
		}
	}
	return false
}
