package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// SanitizeUrlSegment returns a filename-shaped string that is safe to embed in
// the trailing path segment of a public URL. It strips characters that would
// fragment the URL (slashes, query/fragment delimiters, backslash), control
// chars (NUL/CR/LF/tab/…), and leading/trailing whitespace. When the result
// would be empty, a generic fallback is used so the URL always has a tail
// segment.
func SanitizeUrlSegment(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			continue
		}
		switch r {
		case '/', '\\', '?', '#':
			continue
		}
		b.WriteRune(r)
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "file"
	}
	return out
}

// ClientIP returns the IP address to use for filter/blacklist/log decisions.
// It always uses net.SplitHostPort to handle IPv6 correctly (the old
// strings.Split(":", …)[0] mangled "[::1]:42" into "[" and broke every IP rule
// against IPv6 clients).
//
// When trustProxy is true, the first hop of X-Forwarded-For (or X-Real-IP as a
// fallback) is preferred. This MUST stay false when the listener is exposed
// directly to the internet — clients can forge those headers freely.
func ClientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		// Forwarded headers are validated as IPs before use: an unparsed string
		// flows into the geoip URL template (SSRF), the geo cache key (memory
		// exhaustion), and the login-throttle bucket key (bypass via rotating
		// XFF). net.ParseIP rejects path traversal, control chars, and IPv6
		// zone identifiers; on failure we fall back to the next source.
		if v := r.Header.Get("X-Forwarded-For"); v != "" {
			if i := strings.IndexByte(v, ','); i >= 0 {
				v = v[:i]
			}
			if ip := net.ParseIP(strings.TrimSpace(v)); ip != nil {
				return ip.String()
			}
		}
		if v := strings.TrimSpace(r.Header.Get("X-Real-IP")); v != "" {
			if ip := net.ParseIP(v); ip != nil {
				return ip.String()
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// RemoteAddr without a port (unusual but defensive) — return as-is.
		return r.RemoteAddr
	}
	return host
}

// GenRandomHash / GenRandomString panic on a crypto/rand read failure. That
// failure mode is "kernel entropy source is gone", at which point silently
// returning a zero-value token would create un-guessable URLs that aren't
// actually un-guessable. Panic is the safer outcome.
func GenRandomHash() string {
	rdata := make([]byte, 64)
	if _, err := rand.Read(rdata); err != nil {
		panic(err)
	}
	hash := sha256.Sum256(rdata)
	return fmt.Sprintf("%x", hash)
}

func GenRandomString(n int) string {
	const lb = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	for i := range buf {
		buf[i] = lb[int(buf[i])%len(lb)]
	}
	return string(buf)
}

func GetExecDir() string {
	exe_path, _ := os.Executable()
	return filepath.Dir(exe_path)
}

func ExecPath(name string) string {
	return filepath.Join(GetExecDir(), name)
}
