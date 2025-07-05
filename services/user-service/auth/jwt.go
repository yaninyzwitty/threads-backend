package auth

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var JwtKey = []byte(os.Getenv("JWT_AUTH_SECRET"))

// setup jwt claims

type Claims struct {
	UserID   int64  `json:"user_id"` // userid of the user
	Email    string `json:"email"`
	Username string `json:"username"`
	FullName string `json:"full_name"`
	jwt.RegisteredClaims
}

// GenerateJWT_Token creates and signs a new JWT token containing user-specific claims.
// The token includes the user's ID, email, username, and full name, and is valid for 15 minutes.
// Returns the signed JWT token string or an error if token generation fails.
func GenerateJWTToken(userID int64, email string, username string, fullName string) (string, error) {

	claims := &Claims{
		UserID:   userID,
		Email:    email,
		Username: username,
		FullName: fullName,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(60 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "threads-go-backend",
			Subject:   "user-token",
			Audience:  jwt.ClaimStrings{"threads-go-backend"},
			ID:        uuid.New().String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(JwtKey)
}

// ValidateJWT_Token parses and verifies a JWT token string, returning the associated claims if the token is valid.
func ValidateJWTToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return JwtKey, nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims) // cast into type
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}
