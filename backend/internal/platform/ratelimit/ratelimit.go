package ratelimit

import (
	"fmt"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Limiter struct {
	mu        sync.Mutex
	burst     int
	every     time.Duration
	trusted   []netip.Prefix
	maxKeys   int
	now       func() time.Time
	tat       map[string]time.Time
	lastSweep time.Time
}

func New(burst int, every time.Duration, trustedProxies []netip.Prefix) *Limiter {
	return &Limiter{
		burst:   burst,
		every:   every,
		trusted: trustedProxies,
		maxKeys: 100_000,
		now:     time.Now,
		tat:     map[string]time.Time{},
	}
}

func (l *Limiter) Allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.sweep(now)

	tat, ok := l.tat[key]
	if !ok && len(l.tat) >= l.maxKeys {
		return false, l.every
	}
	if tat.Before(now) {
		tat = now
	}
	tolerance := time.Duration(l.burst-1) * l.every
	if wait := tat.Sub(now) - tolerance; wait > 0 {
		return false, wait
	}
	l.tat[key] = tat.Add(l.every)
	return true, 0
}

func (l *Limiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < time.Minute {
		return
	}
	l.lastSweep = now
	for k, tat := range l.tat {
		if !tat.After(now) {
			delete(l.tat, k)
		}
	}
}

func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ok, wait := l.Allow(ClientIP(r, l.trusted))
		if !ok {
			w.Header().Set("Retry-After", strconv.Itoa(int((wait+time.Second-1)/time.Second)))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"too many requests"}` + "\n"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func ClientIP(r *http.Request, trusted []netip.Prefix) string {
	ap, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	addr := ap.Addr().Unmap()
	if contains(trusted, addr) {
		if fwd := forwardedClient(r.Header.Values("X-Forwarded-For"), trusted); fwd.IsValid() {
			addr = fwd
		}
	}
	if addr.Is6() {
		p, _ := addr.Prefix(64)
		return p.String()
	}
	return addr.String()
}

func contains(prefixes []netip.Prefix, a netip.Addr) bool {
	for _, p := range prefixes {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

func forwardedClient(headers []string, trusted []netip.Prefix) netip.Addr {
	var hops []string
	for _, h := range headers {
		hops = append(hops, strings.Split(h, ",")...)
	}
	for i := len(hops) - 1; i >= 0; i-- {
		a, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if err != nil {
			return netip.Addr{}
		}
		if a = a.Unmap(); !contains(trusted, a) {
			return a
		}
	}
	return netip.Addr{}
}

func ParseTrustedProxies(s string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if p, err := netip.ParsePrefix(part); err == nil {
			out = append(out, p.Masked())
			continue
		}
		a, err := netip.ParseAddr(part)
		if err != nil {
			return nil, fmt.Errorf("invalid IP or CIDR %q", part)
		}
		a = a.Unmap()
		out = append(out, netip.PrefixFrom(a, a.BitLen()))
	}
	return out, nil
}
