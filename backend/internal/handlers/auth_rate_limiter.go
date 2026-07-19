package handlers

import (
	"strconv"
	"strings"
	"sync"
	"time"
)

type loginRateLimiter struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	attempts map[string]loginAttempt
	now      func() time.Time
}

type loginAttempt struct {
	count     int
	expiresAt time.Time
}

type loginRateLimitResult struct {
	Limited    bool
	RetryAfter time.Duration
}

func newLoginRateLimiter(limit int, window time.Duration) *loginRateLimiter {
	if limit <= 0 {
		limit = 5
	}
	if window <= 0 {
		window = 10 * time.Minute
	}
	return &loginRateLimiter{
		limit:    limit,
		window:   window,
		attempts: make(map[string]loginAttempt),
		now:      time.Now,
	}
}

func (l *loginRateLimiter) Check(ip, username string) loginRateLimitResult {
	if l == nil {
		return loginRateLimitResult{}
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	key := loginRateLimitKey(ip, username)
	attempt, ok := l.attempts[key]
	now := l.now()
	if !ok || !attempt.expiresAt.After(now) {
		if ok {
			delete(l.attempts, key)
		}
		return loginRateLimitResult{}
	}
	if attempt.count < l.limit {
		return loginRateLimitResult{}
	}
	return loginRateLimitResult{Limited: true, RetryAfter: attempt.expiresAt.Sub(now)}
}

func (l *loginRateLimiter) AddFailure(ip, username string) {
	if l == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	key := loginRateLimitKey(ip, username)
	now := l.now()
	attempt, ok := l.attempts[key]
	if !ok || !attempt.expiresAt.After(now) {
		attempt = loginAttempt{expiresAt: now.Add(l.window)}
	}
	attempt.count++
	l.attempts[key] = attempt
}

func (l *loginRateLimiter) Reset(ip, username string) {
	if l == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, loginRateLimitKey(ip, username))
}

func loginRateLimitKey(ip, username string) string {
	return strings.TrimSpace(ip) + "|" + strings.ToLower(strings.TrimSpace(username))
}

func retryAfterSeconds(duration time.Duration) string {
	seconds := int(duration.Seconds())
	if seconds < 1 {
		seconds = 1
	}
	return strconv.Itoa(seconds)
}
