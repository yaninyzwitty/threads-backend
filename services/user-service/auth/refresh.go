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

func (r *RefreshTokenStore) CreateRefreshToken(ctx context.Context, userID string) (string, error) {
	token, err := generateRefreshToken()
	if err != nil {
		return "", err
	}

	key := fmt.Sprintf("refresh:%s:%s", userID, token)
	err = r.Redis.Set(ctx, key, "valid", 7*24*time.Hour).Err()
	if err != nil {
		return "", err
	}

	return token, nil

}

// Validate Refresh Token

func (r *RefreshTokenStore) ValidateRefreshToken(ctx context.Context, userID, token string) (bool, error) {
	key := fmt.Sprintf("refresh:%s:%s", userID, token)

	res, err := r.Redis.Get(ctx, key).Result()

	if err == redis.Nil {
		return false, nil
	}

	return res == "valid", nil
}

// Delete Refresh Token (Logout)

func (r *RefreshTokenStore) DeleteRefreshToken(ctx context.Context, userID, token string) error {
	key := fmt.Sprintf("refresh:%s:%s", userID, token)
	return r.Redis.Del(ctx, key).Err()

}
