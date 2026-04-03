package security

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiter implements a rate limiter with Redis backend and in-memory fallback
type RateLimiter struct {
	redis    *redis.Client
	fallback *inMemoryLimiter
	limit    int
	window   time.Duration
	logger   *slog.Logger
}

// inMemoryLimiter is a simple in-memory rate limiter used as fallback
type inMemoryLimiter struct {
	mu       sync.RWMutex
	requests map[string]*ipData
	limit    int
	window   time.Duration
}

type ipData struct {
	count     int
	resetTime time.Time
}

// NewRateLimiter creates a new rate limiter with optional Redis backend.
// If redisURL is empty or Redis connection fails, it falls back to in-memory limiting.
func NewRateLimiter(redisURL string, limitPerHour int, logger *slog.Logger) *RateLimiter {
	rl := &RateLimiter{
		limit:  limitPerHour,
		window: time.Hour,
		logger: logger,
		fallback: &inMemoryLimiter{
			requests: make(map[string]*ipData),
			limit:    limitPerHour,
			window:   time.Hour,
		},
	}

	// Start cleanup goroutine for in-memory fallback
	go rl.fallback.cleanup()

	// Try to connect to Redis if URL is provided
	if redisURL != "" {
		opt, err := redis.ParseURL(redisURL)
		if err != nil {
			logger.Warn("failed to parse Redis URL, using in-memory rate limiting", "error", err)
			return rl
		}

		client := redis.NewClient(opt)

		// Test connection
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := client.Ping(ctx).Err(); err != nil {
			logger.Warn("failed to connect to Redis, using in-memory rate limiting", "error", err)
			return rl
		}

		rl.redis = client
		logger.Info("connected to Redis for rate limiting")
	} else {
		logger.Info("no Redis URL configured, using in-memory rate limiting")
	}

	return rl
}

// Allow checks if the IP is allowed to make a request
// Returns true if allowed, false if rate limited
func (rl *RateLimiter) Allow(ip string) bool {
	if rl.redis != nil {
		allowed, err := rl.allowRedis(ip)
		if err != nil {
			rl.logger.Warn("Redis error, falling back to in-memory", "error", err, "ip", ip)
			return rl.fallback.allow(ip)
		}
		return allowed
	}
	return rl.fallback.allow(ip)
}

// allowRedis implements rate limiting using Redis
func (rl *RateLimiter) allowRedis(ip string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	key := fmt.Sprintf("ratelimit:%s", ip)

	// Use a Lua script for atomic increment with expiry
	script := redis.NewScript(`
		local current = redis.call("INCR", KEYS[1])
		if current == 1 then
			redis.call("EXPIRE", KEYS[1], ARGV[1])
		end
		return current
	`)

	windowSeconds := int(rl.window.Seconds())
	result, err := script.Run(ctx, rl.redis, []string{key}, windowSeconds).Int()
	if err != nil {
		return false, err
	}

	return result <= rl.limit, nil
}

// RemainingRequests returns the number of remaining requests for an IP
func (rl *RateLimiter) RemainingRequests(ip string) int {
	if rl.redis != nil {
		remaining, err := rl.remainingRedis(ip)
		if err != nil {
			rl.logger.Warn("Redis error, falling back to in-memory", "error", err, "ip", ip)
			return rl.fallback.remainingRequests(ip)
		}
		return remaining
	}
	return rl.fallback.remainingRequests(ip)
}

// remainingRedis gets remaining requests from Redis
func (rl *RateLimiter) remainingRedis(ip string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	key := fmt.Sprintf("ratelimit:%s", ip)

	count, err := rl.redis.Get(ctx, key).Int()
	if err == redis.Nil {
		return rl.limit, nil
	}
	if err != nil {
		return 0, err
	}

	remaining := rl.limit - count
	if remaining < 0 {
		return 0, nil
	}
	return remaining, nil
}

// Close closes the Redis connection if open
func (rl *RateLimiter) Close() error {
	if rl.redis != nil {
		return rl.redis.Close()
	}
	return nil
}

// In-memory fallback methods

func (m *inMemoryLimiter) allow(ip string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	data, exists := m.requests[ip]
	if !exists {
		m.requests[ip] = &ipData{
			count:     1,
			resetTime: now.Add(m.window),
		}
		return true
	}

	// Check if window has expired
	if now.After(data.resetTime) {
		data.count = 1
		data.resetTime = now.Add(m.window)
		return true
	}

	// Check if under limit
	if data.count < m.limit {
		data.count++
		return true
	}

	return false
}

func (m *inMemoryLimiter) remainingRequests(ip string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, exists := m.requests[ip]
	if !exists {
		return m.limit
	}

	if time.Now().After(data.resetTime) {
		return m.limit
	}

	remaining := m.limit - data.count
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (m *inMemoryLimiter) cleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.Lock()
		now := time.Now()
		for ip, data := range m.requests {
			if now.After(data.resetTime) {
				delete(m.requests, ip)
			}
		}
		m.mu.Unlock()
	}
}
