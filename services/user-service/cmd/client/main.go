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

// initialize client here

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

	req := connect.NewRequest(&userv1.FollowUserRequest{
		UserId:      int64(143916461868469878),
		FollowingId: int64(143916359158353526),
	})
	req.Header().Set("Authorization", "Bearer "+os.Getenv("ZURI_TOKEN"))

	res, err := userServiceClient.FollowUser(context.Background(), req)

	if err != nil {
		slog.Error("failed to follow user", "error", err)
		os.Exit(1)
	}

	slog.Info("user created", "user", res.Msg.Success)

}
