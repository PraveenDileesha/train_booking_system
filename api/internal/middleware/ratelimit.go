// Package middleware holds HTTP middleware shared across handlers.
package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// IPRateLimiter gives each client IP its own token bucket.
// This is an in-process limiter, correct and sufficient for a single API instance.
// If this service ever runs as multiple replicas behind a load balancer, per-instance limits stop being globally accurate and the state would need to move to a shared store such as Redis, which isn't needed at this scale.
type IPRateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rate     rate.Limit
	burst    int
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewIPRateLimiter allows r requests/second sustained per IP and bursts up to burst.
// It starts a background goroutine that evicts IPs that have been quiet for a while, so the visitor map doesn't grow unbounded.
func NewIPRateLimiter(r rate.Limit, burst int) *IPRateLimiter {
	l := &IPRateLimiter{
		visitors: make(map[string]*visitor),
		rate:     r,
		burst:    burst,
	}
	go l.evictStaleVisitors()
	return l
}

func (l *IPRateLimiter) getLimiter(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()

	v, ok := l.visitors[ip]
	if !ok {
		limiter := rate.NewLimiter(l.rate, l.burst)
		l.visitors[ip] = &visitor{limiter: limiter, lastSeen: time.Now()}
		return limiter
	}
	v.lastSeen = time.Now()
	return v.limiter
}

func (l *IPRateLimiter) evictStaleVisitors() {
	for {
		time.Sleep(time.Minute)
		l.mu.Lock()
		for ip, v := range l.visitors {
			if time.Since(v.lastSeen) > 3*time.Minute {
				delete(l.visitors, ip)
			}
		}
		l.mu.Unlock()
	}
}

func clientIP(r *http.Request) string {
	// This service isn't deployed behind a reverse proxy, so RemoteAddr is the real client address and no X-Forwarded-For handling is needed.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// Limit wraps a handler so requests exceeding the per-IP rate get a 429 instead of reaching it.
func (l *IPRateLimiter) Limit(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.getLimiter(clientIP(r)).Allow() {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	})
}
