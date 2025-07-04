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

// AuthInterceptor parses JWT and adds user to context

func AuthInterceptor() connect.Interceptor {
	return connect.UnaryInterceptorFunc(func(uf connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {

			// Define public,
			publicRoutes := map[string]struct{}{
				"CreateUser":   {},
				"LoginUser":    {},
				"RefreshToken": {},
			}

			fullMethod := strings.TrimSpace(req.Spec().Procedure) // e.g., "/user.v1.UserService/LoginUser"
			methodParts := strings.Split(fullMethod, "/")

			if len(methodParts) < 2 {
				return nil, connect.NewError(connect.CodeInternal, errors.New("invalid method format"))
			}

			methodName := methodParts[len(methodParts)-1] // e.g., "LoginUser"

			// Check if it's a public route
			if _, ok := publicRoutes[methodName]; ok {
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

			claims, err := ValidateJWTToken(token)
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

// Helper function to get user from context
func GetUserFromContext(ctx context.Context) (*userv1.User, error) {
	user, ok := ctx.Value(UserContextKey).(*userv1.User)
	if !ok {
		return nil, errors.New("user not found in context")
	}
	return user, nil
}
