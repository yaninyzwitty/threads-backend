package helpers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Context key type for type safety
type contextKey string

const (
	EmailContextKey  contextKey = "email"
	UserIDContextKey contextKey = "userID"
	RoleContextKey   contextKey = "role"
)

// Claims represents the JWT claims structure
type Claims struct {
	Email string `json:"email,omitempty"`
	Role  string `json:"role,omitempty"`
	jwt.RegisteredClaims
}

// Globals
var (
	redisClient *redis.Client
	jwtKey      []byte
)

// -------------------- Initialization --------------------

// Initialize helpers (Redis, JWT)
func InitHelpers() {
	redisURL := GetEnvOrDefault("REDIS_URL", "")
	if redisURL == "" {
		slog.Error("REDIS_URL environment variable not set")
		os.Exit(1)
	}

	// Redis connection
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		slog.Error("invalid REDIS_URL format", "error", err)
		os.Exit(1)
	}

	redisClient = redis.NewClient(opts)

	// JWT key
	secret := GetEnvOrDefault("JWT_AUTH_SECRET", "")
	if secret == "" {
		slog.Error("JWT_AUTH_SECRET is not set")
		os.Exit(1)
	}
	jwtKey = []byte(secret)

	slog.Info("Helpers initialized (Redis, JWT)")
}

// -------------------- JWT Generation --------------------

func GenerateToken(email, role string, userID int64) (string, error) {
	claims := &Claims{
		Email: email,
		Role:  role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			Issuer:    "threads-go-backend",
			Subject:   strconv.FormatInt(userID, 10), // User ID in 'sub'
			Audience:  jwt.ClaimStrings{"threads-go-backend"},
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			ID:        uuid.New().String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtKey)
}

// ValidateJWT parses and verifies a JWT token string, returning its claims if valid.
// Returns an error if the token is empty, invalid, uses an unexpected signing method, or lacks required claims.

func ValidateJWT(tokenString string) (*Claims, error) {
	if tokenString == "" {
		return nil, errors.New("empty token string")
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return jwtKey, nil
	})
	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, errors.New("invalid claims type")
	}

	if claims.Email == "" || claims.Subject == "" {
		return nil, errors.New("missing email or subject in claims")
	}

	return claims, nil
}

// -------------------- Redis Blacklist --------------------

func BlacklistToken(ctx context.Context, token string, expiration time.Time) error {
	ttl := time.Until(expiration)
	return redisClient.Set(ctx, token, "blacklisted", ttl).Err()
}

func IsTokenBlacklisted(ctx context.Context, token string) bool {
	val, err := redisClient.Get(ctx, token).Result()
	if err == redis.Nil {
		return false
	} else if err != nil {
		slog.Error("Error checking token in Redis", "error", err)
		return false
	}

	return val == "blacklisted"
}

// -------------------- Auth Interceptor --------------------

func AuthInterceptor() connect.Interceptor {
	return connect.UnaryInterceptorFunc(func(uf connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {

			// here we allow unathenticated access to create user method
			if strings.HasSuffix(req.Spec().Procedure, "CreateUser") {
				// skip auth for CreateUser
				return uf(ctx, req)
			}
			authHeader := req.Header().Get("Authorization")
			if authHeader == "" {
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing authorization header"))
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid authorization header format"))
			}

			token := parts[1]
			if token == "" {
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("empty token"))
			}

			if IsTokenBlacklisted(ctx, token) {
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("token is blacklisted"))
			}

			claims, err := ValidateJWT(token)
			if err != nil {
				return nil, connect.NewError(connect.CodeUnauthenticated, err)
			}

			ctx = context.WithValue(ctx, EmailContextKey, claims.Email)
			ctx = context.WithValue(ctx, UserIDContextKey, claims.Subject)
			ctx = context.WithValue(ctx, RoleContextKey, claims.Role)

			return uf(ctx, req)
		}
	})
}

// -------------------- Context Helpers --------------------

func GetEmailFromContext(ctx context.Context) (string, bool) {
	email, ok := ctx.Value(EmailContextKey).(string)
	return email, ok
}

func GetUserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(UserIDContextKey).(string)
	return userID, ok
}

func GetRoleFromContext(ctx context.Context) (string, bool) {
	role, ok := ctx.Value(RoleContextKey).(string)
	return role, ok
}
