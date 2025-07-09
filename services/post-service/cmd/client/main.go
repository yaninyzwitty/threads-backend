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

	req := connect.NewRequest(&postsv1.CreatePostRequest{
		Content: "May you go have to encounter The Spirit of Truth and power of God💡",
		ImageUrl: `https://scontent-mba2-1.cdninstagram.com/o1/v/t2/f2/m86/AQNm5cBEf4veFEtNemRm_BbjWtZRPuWjnAB583btNs-2pnD07bod28F_nziahODsF1eS9Y1CS6-0QBcCiMoPBjjY9u5y_bhKuT5ioIk.mp4?_nc_cat=101&_nc_sid=5e9851&_nc_ht=scontent-mba2-1.cdninstagram.com&_nc_ohc=H-ngvpH8nRIQ7kNvwEFKL36&efg=eyJ2ZW5jb2RlX3RhZyI6Inhwdl9wcm9ncmVzc2l2ZS5JTlNUQUdSQU0uRkVFRC5DMy43MjAuZGFzaF9iYXNlbGluZV8xX3YxIiwieHB2X2Fzc2V0X2lkIjoxMDcxNjEyNDk4MzY1Nzk5LCJ2aV91c2VjYXNlX2lkIjoxMDA5OSwiZHVyYXRpb25fcyI6MTczLCJ1cmxnZW5fc291cmNlIjoid3d3In0%3D&ccb=17-1&vs=bc129a65b09d3a17&_nc_vs=HBksFQIYUmlnX3hwdl9yZWVsc19wZXJtYW5lbnRfc3JfcHJvZC83QzRDRDE5REUyRkEwOEJBNkQxQUEyQzIxNjZDRjI5RF92aWRlb19kYXNoaW5pdC5tcDQVAALIARIAFQIYOnBhc3N0aHJvdWdoX2V2ZXJzdG9yZS9HSWRvcVI3SHJfLWg0WllPQU9Ebl9mcHNiWGcwYnFfRUFBQUYVAgLIARIAKAAYABsCiAd1c2Vfb2lsATEScHJvZ3Jlc3NpdmVfcmVjaXBlATEVAAAmzrXqpIeo5wMVAigCQzMsF0BlrMzMzMzNGBJkYXNoX2Jhc2VsaW5lXzFfdjERAHXqB2XmnQEA&_nc_zt=28&oh=00_AfTdN5lyjQSjsXXjAfOjZkuWhFcGvm6Eu7gISOFLwY9sGg&oe=686F5476
`,
		UserId: 145453184356208246,
	})

	req.Header().Set("Authorization", "Bearer "+os.Getenv("ACCESS_TOKEN"))

	res, err := postServiceClient.CreatePost(context.TODO(), req)
	if err != nil {
		slog.Error("failed to create post", "error", err)
		os.Exit(1)
	}
	slog.Info("post created", "post_id", res.Msg.Post.Id)

}
