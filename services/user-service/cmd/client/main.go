package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"connectrpc.com/connect"
	"github.com/joho/godotenv"
	userv1 "github.com/yaninyzwitty/threads-go-backend/gen/user/v1"
	"github.com/yaninyzwitty/threads-go-backend/gen/user/v1/userv1connect"
	"github.com/yaninyzwitty/threads-go-backend/shared/pkg"
)

// main initializes configuration, sets up a user service client, and creates a test user via RPC.
// The function loads environment variables and configuration, constructs the user service client, and sends a create user request. On failure, it logs the error and exits; on success, it logs the created user information.

func main() {

	if err := godotenv.Load(); err != nil {
		slog.Error("failed to load .env", "error", err)
		os.Exit(1)
	}

	cfg := pkg.Config{}

	if err := cfg.LoadConfig("config.yaml"); err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// set up http client
	httpClient := http.DefaultClient
	userServiceUrl := fmt.Sprintf("http://localhost:%d", cfg.UserServer.Port)

	slog.Info("user service url", "url", userServiceUrl)

	userServiceClient := userv1connect.NewUserServiceClient(
		httpClient,
		userServiceUrl,
	)
	req := connect.NewRequest(&userv1.UnfollowUserRequest{
		FollowingId: 144604909692538486,
	})

	req.Header().Set("Authorization", "Bearer "+os.Getenv("ACCESS_TOKEN"))

	res, err := userServiceClient.UnfollowUser(context.Background(), req)

	if err != nil {
		slog.Error("failed to follow user ", "error", err)
		os.Exit(1)
	}

	slog.Info("user followed", "bool", res.Msg.Success)

}
