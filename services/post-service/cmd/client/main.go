package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"connectrpc.com/connect"
	"github.com/joho/godotenv"
	postsv1 "github.com/yaninyzwitty/threads-go-backend/gen/posts/v1"
	"github.com/yaninyzwitty/threads-go-backend/gen/posts/v1/postsv1connect"
	"github.com/yaninyzwitty/threads-go-backend/shared/pkg"
)

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
	postServiceUrl := fmt.Sprintf("http://localhost:%d", cfg.PostServer.Port)

	slog.Info("post service url", "url", postServiceUrl)

	postServiceClient := postsv1connect.NewPostServiceClient(
		httpClient,
		postServiceUrl,
	)

	req := connect.NewRequest(&postsv1.GetPostRequest{
		PostId: 145604212619759222,
	})

	req.Header().Set("Authorization", "Bearer "+os.Getenv("ACCESS_TOKEN"))

	res, err := postServiceClient.GetPostWithMetadata(context.TODO(), req)
	if err != nil {
		slog.Error("failed to get post", "error", err)
		os.Exit(1)
	}
	slog.Info("post retrieved", "post_id", res.Msg)

}
