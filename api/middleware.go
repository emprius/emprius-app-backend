package api

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	// lastSeenUpdateThreshold is the minimum time between lastSeen updates for a user
	lastSeenUpdateThreshold = 5 * time.Minute
)

// lastSeenCache holds the last update time for each user to implement throttling
var lastSeenCache = sync.Map{}

// lastSeenMiddleware updates the user's lastSeen timestamp for authenticated requests.
// It uses a 5-minute throttle to reduce database load and performs updates asynchronously.
func (a *API) lastSeenMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract user ID from the request header (set by authenticator middleware)
		userID := r.Header.Get("X-User-Id")
		if userID != "" {
			// Check if we should update lastSeen for this user
			shouldUpdate := false
			now := time.Now()

			// Load or initialize the last update time for this user
			if lastUpdateTime, ok := lastSeenCache.Load(userID); ok {
				if lastUpdate, ok := lastUpdateTime.(time.Time); ok {
					// Only update if enough time has passed
					if now.Sub(lastUpdate) >= lastSeenUpdateThreshold {
						shouldUpdate = true
					}
				}
			} else {
				// First request for this user in cache, update
				shouldUpdate = true
			}

			if shouldUpdate {
				// Update cache immediately to prevent concurrent updates
				lastSeenCache.Store(userID, now)

				// Perform database update asynchronously
				go func(uid string, timestamp time.Time) {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()

					objID, err := primitive.ObjectIDFromHex(uid)
					if err != nil {
						log.Error().Err(err).Str("userId", uid).Msg("Failed to parse user ID for lastSeen update")
						return
					}

					update := bson.M{"lastSeen": timestamp}
					_, err = a.database.UserService.UpdateUser(ctx, objID, update)
					if err != nil {
						log.Error().Err(err).Str("userId", uid).Msg("Failed to update lastSeen timestamp")
						// Remove from cache on failure so it will retry on next request
						lastSeenCache.Delete(uid)
					}
				}(userID, now)
			}
		}

		// Continue with the request
		next.ServeHTTP(w, r)
	})
}
