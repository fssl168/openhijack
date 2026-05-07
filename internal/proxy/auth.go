package proxy

import (
	"crypto/subtle"
	"log/slog"
	"net"
	"sync"
	"time"

	"openhijack/internal/errors"
)

type ProxyAuth struct {
	AuthKey       string
	EnableAuth    bool
	rateLimiter   *RateLimiter
	logger        *slog.Logger
}

type RateLimiter struct {
	mu         sync.Mutex
	buckets    map[string]*tokenBucket
	windowSize time.Duration
	maxRequests int
	cleanupInterval time.Duration
	stopChan   chan struct{}
}

type tokenBucket struct {
	tokens     int
	lastRefill time.Time
}

const (
	defaultMaxRequests   = 60
	defaultWindowSize     = time.Minute
	defaultCleanupInterval = 5 * time.Minute
)

func NewProxyAuth(key string) *ProxyAuth {
	auth := &ProxyAuth{
		AuthKey:    key,
		EnableAuth: key != "",
		logger:     slog.Default(),
	}
	
	if key != "" {
		auth.rateLimiter = NewRateLimiter(defaultMaxRequests, defaultWindowSize)
		go auth.rateLimiter.StartCleanup()
	}
	
	return auth
}

func NewRateLimiter(maxRequests int, windowSize time.Duration) *RateLimiter {
	return &RateLimiter{
		buckets:          make(map[string]*tokenBucket),
		windowSize:       windowSize,
		maxRequests:      maxRequests,
		cleanupInterval: defaultCleanupInterval,
		stopChan:         make(chan struct{}),
	}
}

func (rl *RateLimiter) Allow(clientIP string) bool {
	if rl == nil {
		return true
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	bucket, exists := rl.buckets[clientIP]
	if !exists {
		rl.buckets[clientIP] = &tokenBucket{
			tokens:     rl.maxRequests - 1,
			lastRefill: now,
		}
		return true
	}

	elapsed := now.Sub(bucket.lastRefill)
	if elapsed >= rl.windowSize {
		bucket.tokens = rl.maxRequests - 1
		bucket.lastRefill = now
		return true
	}

	refillTokens := int(float64(rl.maxRequests) * (elapsed.Seconds() / rl.windowSize.Seconds()))
	bucket.tokens += refillTokens
	if bucket.tokens > rl.maxRequests {
		bucket.tokens = rl.maxRequests
	}
	bucket.lastRefill = now

	if bucket.tokens > 0 {
		bucket.tokens--
		return true
	}

	return false
}

func (rl *RateLimiter) StartCleanup() {
	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.cleanup()
		case <-rl.stopChan:
			return
		}
	}
}

func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for ip, bucket := range rl.buckets {
		if now.Sub(bucket.lastRefill) > rl.windowSize*2 {
			delete(rl.buckets, ip)
		}
	}
}

func (rl *RateLimiter) Stop() {
	if rl != nil && rl.stopChan != nil {
		close(rl.stopChan)
	}
}

func (a *ProxyAuth) SetLogger(logger *slog.Logger) {
	a.logger = logger
}

func (a *ProxyAuth) Verify(authHeader string, clientIP string) error {
	if !a.EnableAuth {
		return errors.NewErrAuthenticationFailed("authentication is required but not configured")
	}

	if a.AuthKey == "" {
		a.logger.Warn("authentication enabled but no auth key configured",
			"client_ip", extractIP(clientIP),
		)
		return errors.NewErrAuthenticationFailed("server authentication misconfigured")
	}

	if authHeader == "" {
		a.logger.Warn("authentication failed: missing authorization header",
			"client_ip", extractIP(clientIP),
		)
		return errors.NewErrAuthenticationFailed("missing authorization header")
	}

	if a.rateLimiter != nil && !a.rateLimiter.Allow(clientIP) {
		a.logger.Warn("authentication failed: rate limit exceeded",
			"client_ip", extractIP(clientIP),
		)
		return errors.NewErrAuthenticationFailed("too many requests, please try again later")
	}

	provided := extractBearerToken(authHeader)

	if subtle.ConstantTimeCompare(
		[]byte(provided),
		[]byte(a.AuthKey),
	) != 1 {
		a.logger.Warn("authentication failed: invalid credentials",
			"client_ip", extractIP(clientIP),
		)
		return errors.NewErrAuthenticationFailed("invalid authentication credentials")
	}

	a.logger.Debug("authentication successful",
		"client_ip", extractIP(clientIP),
	)
	return nil
}

func (a *ProxyAuth) VerifyLegacy(authHeader string) bool {
	if a.AuthKey == "" {
		return false
	}
	if authHeader == "" {
		return false
	}
	provided := extractBearerToken(authHeader)
	return subtle.ConstantTimeCompare([]byte(provided), []byte(a.AuthKey)) == 1
}

func extractBearerToken(authHeader string) string {
	token := authHeader
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	} else if len(token) > 7 && token[:7] == "bearer " {
		token = token[7:]
	}
	return token
}

func extractIP(clientAddr string) string {
	host, _, err := net.SplitHostPort(clientAddr)
	if err != nil {
		return clientAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return host
	}
	if ip.To4() != nil {
		return ip.String()
	}
	return ip.String()
}

func (a *ProxyAuth) Close() error {
	if a.rateLimiter != nil {
		a.rateLimiter.Stop()
	}
	return nil
}

type AuthStats struct {
	TotalAttempts  int64
	SuccessCount  int64
	FailureCount  int64
	RateLimitedCount int64
	BucketsCount  int
}

func (a *ProxyAuth) GetStats() AuthStats {
	stats := AuthStats{}
	if a.rateLimiter != nil {
		a.rateLimiter.mu.Lock()
		stats.BucketsCount = len(a.rateLimiter.buckets)
		a.rateLimiter.mu.Unlock()
	}
	return stats
}
