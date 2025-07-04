package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RefreshTokenStore struct {
	Redis *redis.Client
}

func NewRefreshTokenStore(redis *redis.Client) *RefreshTokenStore {
	return &RefreshTokenStore{Redis: redis}
}

// Generate random refresh token string

func generateRefreshToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil

}

// Create and store Refresh Token

func (r *RefreshTokenStore) CreateRefreshToken(ctx context.Context, userID int64) (string, error) {
	token, err := generateRefreshToken()
	if err != nil {
		return "", err
	}

	key := fmt.Sprintf("refresh:%d:%s", userID, token)
	err = r.Redis.Set(ctx, key, "valid", 7*24*time.Hour).Err()
	if err != nil {
		return "", err
	}

	return token, nil

}

// Store only one refresh token per user
func (r *RefreshTokenStore) CreateOrGetRefreshToken(ctx context.Context, userID int64) (string, error) {
	key := fmt.Sprintf("refresh:%d", userID)

	// Try to get existing token
	val, err := r.Redis.Get(ctx, key).Result()
	if err != nil && err != redis.Nil {
		return "", err
	}

	if err == nil {
		// token exists
		return val, nil
	}

	// Else create and set
	token, err := generateRefreshToken()
	if err != nil {
		return "", err
	}

	err = r.Redis.Set(ctx, key, token, 7*24*time.Hour).Err()
	if err != nil {
		return "", err
	}

	return token, nil
}

func (r *RefreshTokenStore) ValidateRefreshToken(ctx context.Context, userID int64, token string) (bool, error) {
	key := fmt.Sprintf("refresh:%d", userID)
	val, err := r.Redis.Get(ctx, key).Result()

	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return val == token, nil
}

func (r *RefreshTokenStore) DeleteRefreshToken(ctx context.Context, userID int64) error {
	key := fmt.Sprintf("refresh:%d", userID)
	return r.Redis.Del(ctx, key).Err()
}
