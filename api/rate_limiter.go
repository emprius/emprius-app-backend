package api

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// RateLimiter provides rate limiting functionality
type RateLimiter struct {
	mu      sync.RWMutex
	buckets map[string]*TokenBucket
}

// TokenBucket represents a token bucket for rate limiting
type TokenBucket struct {
	tokens     int
	maxTokens  int
	refillRate int // tokens per minute
	lastRefill time.Time
	mu         sync.Mutex
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{
		buckets: make(map[string]*TokenBucket),
	}

	// Start cleanup goroutine to remove old buckets
	go rl.cleanup()

	return rl
}

// NewTokenBucket creates a new token bucket
func NewTokenBucket(maxTokens, refillRate int) *TokenBucket {
	return &TokenBucket{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// Allow checks if an action is allowed for the given key
func (rl *RateLimiter) Allow(key string, maxTokens, refillRate int) bool {
	rl.mu.Lock()
	bucket, exists := rl.buckets[key]
	if !exists {
		bucket = NewTokenBucket(maxTokens, refillRate)
		rl.buckets[key] = bucket
	}
	rl.mu.Unlock()

	return bucket.Allow()
}

// Allow checks if a token can be consumed from the bucket
func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastRefill)

	// Refill tokens based on elapsed time
	if elapsed > 0 {
		tokensToAdd := int(elapsed.Minutes()) * tb.refillRate
		if tokensToAdd > 0 {
			tb.tokens += tokensToAdd
			if tb.tokens > tb.maxTokens {
				tb.tokens = tb.maxTokens
			}
			tb.lastRefill = now
		}
	}

	// Check if we have tokens available
	if tb.tokens > 0 {
		tb.tokens--
		return true
	}

	return false
}

// cleanup removes old unused buckets
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for key, bucket := range rl.buckets {
			bucket.mu.Lock()
			if now.Sub(bucket.lastRefill) > 30*time.Minute {
				delete(rl.buckets, key)
			}
			bucket.mu.Unlock()
		}
		rl.mu.Unlock()
	}
}

// MessageRateLimiter provides rate limiting specifically for messages
type MessageRateLimiter struct {
	limiter *RateLimiter
}

// NewMessageRateLimiter creates a new message rate limiter
func NewMessageRateLimiter() *MessageRateLimiter {
	return &MessageRateLimiter{
		limiter: NewRateLimiter(),
	}
}

// CheckMessageLimit checks if a user can send a message
func (mrl *MessageRateLimiter) CheckMessageLimit(ctx context.Context, userID primitive.ObjectID, messageType string) error {
	userKey := userID.Hex()

	// Different limits for different message types
	var maxTokens, refillRate int
	var limitType string

	switch messageType {
	case MessageTypePrivate:
		maxTokens = 30  // 30 messages
		refillRate = 30 // refill 30 per minute (1 every 2 seconds)
		limitType = "private messages"
	case MessageTypeCommunity:
		maxTokens = 20  // 20 messages
		refillRate = 20 // refill 20 per minute (1 every 3 seconds)
		limitType = "community messages"
	case "general":
		maxTokens = 10  // 10 messages
		refillRate = 10 // refill 10 per minute (1 every 6 seconds)
		limitType = "general forum messages"
	default:
		maxTokens = 10
		refillRate = 10
		limitType = "messages"
	}

	// Create a composite key for user + message type
	key := fmt.Sprintf("%s:%s", userKey, messageType)

	if !mrl.limiter.Allow(key, maxTokens, refillRate) {
		return fmt.Errorf("rate limit exceeded for %s. Please wait before sending more messages", limitType)
	}

	return nil
}
