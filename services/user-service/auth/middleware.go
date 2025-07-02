package auth

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	userv1 "github.com/yaninyzwitty/threads-go-backend/gen/user/v1"
)

type contextKey string

const UserContextKey contextKey = "user"

// AuthInterceptor returns a Connect interceptor that enforces JWT authentication for protected routes.
// It parses the "Authorization" header, validates the JWT, and attaches the authenticated user to the request context.
// Unauthenticated access is allowed for the "CreateUser" and "RefreshToken" procedures.

func AuthInterceptor() connect.Interceptor {
	return connect.UnaryInterceptorFunc(func(uf connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {

			publicRoutes := []string{"CreateUser", "RefreshToken"}

			// Allow unauthenticated access to CreateUser and RefreshToken
			for _, route := range publicRoutes {
				if strings.HasSuffix(req.Spec().Procedure, route) {
					return uf(ctx, req)
				}
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

			claims, err := ValidateJWT_Token(token)
			if err != nil {
				return nil, connect.NewError(connect.CodeUnauthenticated, err)
			}

			user := &userv1.User{
				Id:       claims.UserID,
				Email:    claims.Email,
				Username: claims.Username,
				FullName: claims.FullName,
			}

			ctx = context.WithValue(ctx, UserContextKey, user)

			return uf(ctx, req)
		}
	})

}

// GetUserFromContext retrieves the user information stored in the context.
// Returns an error if no user is found in the context.
func GetUserFromContext(ctx context.Context) (*userv1.User, error) {
	user, ok := ctx.Value(UserContextKey).(*userv1.User)
	if !ok {
		return nil, errors.New("user not found in context")
	}
	return user, nil
}
