package controller

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	postsv1 "github.com/yaninyzwitty/threads-go-backend/gen/posts/v1"
	"github.com/yaninyzwitty/threads-go-backend/services/post-service/repository"
	"github.com/yaninyzwitty/threads-go-backend/services/user-service/auth"
	"github.com/yaninyzwitty/threads-go-backend/shared/snowflake"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type PostController struct {
	postsRepo *repository.PostRepository
}

func NewPostController(postsRepo *repository.PostRepository) *PostController {
	return &PostController{
		postsRepo: postsRepo,
	}
}

func (c *PostController) CreatePost(ctx context.Context, req *connect.Request[postsv1.CreatePostRequest]) (*connect.Response[postsv1.CreatePostResponse], error) {
	if req.Msg.Content == "" || req.Msg.ImageUrl == "" || req.Msg.UserId == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid request"))
	}

	// TODO-remove user id from arguments

	// get user from context
	user, err := auth.GetUserFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
	}

	if user.Id != req.Msg.UserId {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("unauthenticated"))
	}

	postId, err := snowflake.GenerateID()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to generate snowflake id: %w", err))
	}

	post := &postsv1.Post{
		Id:           int64(postId),
		Content:      req.Msg.GetContent(),
		ImageUrl:     req.Msg.GetImageUrl(),
		UserId:       user.Id,
		CreatedAt:    timestamppb.Now(),
		LikeCount:    0,
		ShareCount:   0,
		CommentCount: 0,
		RepostCount:  0,
	}

	if err := c.postsRepo.CreatePost(ctx, post); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create post: %w", err))
	}

	return connect.NewResponse(&postsv1.CreatePostResponse{
		Post: post,
	}), nil

}

func (c *PostController) GetPost(
	ctx context.Context,
	req *connect.Request[postsv1.GetPostRequest],
) (*connect.Response[postsv1.GetPostResponse], error) {
	if req.Msg.GetPostId() == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("post_id is required"))
	}

	post, err := c.postsRepo.GetPost(ctx, req.Msg.GetPostId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if post == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("post not found"))
	}

	return connect.NewResponse(&postsv1.GetPostResponse{
		Post: post,
	}), nil
}

func (c *PostController) ListPostsByUser(
	ctx context.Context,
	req *connect.Request[postsv1.ListPostsByUserRequest],
) (*connect.Response[postsv1.ListPostsByUserResponse], error) {
	if req.Msg.GetUserId() == 0 || req.Msg.GetPageSize() <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid user_id or page_size"))
	}

	response, err := c.postsRepo.ListPostsByUser(
		ctx,
		req.Msg.GetUserId(),
		req.Msg.GetPageSize(),
		req.Msg.GetPagingState(),
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(response), nil
}

func (c *PostController) DeletePost(
	ctx context.Context,
	req *connect.Request[postsv1.DeletePostRequest],
) (*connect.Response[postsv1.DeletePostResponse], error) {
	if req.Msg.GetPostId() == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("post_id is required"))
	}

	// Authenticate user
	user, err := auth.GetUserFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
	}

	post, err := c.postsRepo.GetPost(ctx, req.Msg.GetPostId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if post == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("post not found"))
	}

	// Authorization: only the creator can delete
	if post.UserId != user.Id {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("permission denied"))
	}

	// Delete the post
	if err := c.postsRepo.DeletePost(ctx, req.Msg.GetPostId()); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&postsv1.DeletePostResponse{
		Success: true,
	}), nil
}
