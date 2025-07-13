package controller

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	postsv1 "github.com/yaninyzwitty/threads-go-backend/gen/posts/v1"
	userv1 "github.com/yaninyzwitty/threads-go-backend/gen/user/v1"
	"github.com/yaninyzwitty/threads-go-backend/services/post-service/repository"
	"github.com/yaninyzwitty/threads-go-backend/services/user-service/auth"
	"github.com/yaninyzwitty/threads-go-backend/shared/snowflake"
	"golang.org/x/sync/errgroup"
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
		Id:       int64(postId),
		Content:  req.Msg.GetContent(),
		ImageUrl: req.Msg.GetImageUrl(),
		User: &userv1.User{
			Id: req.Msg.UserId,
		},
		CreatedAt: timestamppb.Now(),
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
	if post.User.Id != user.Id {
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

func (c *PostController) InitializePostEngagements(
	ctx context.Context,
	req *connect.Request[postsv1.InitializePostEngagementsRequest],
) (*connect.Response[postsv1.InitializePostEngagementsResponse], error) {
	if req.Msg.PostId == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("post id is required"))
	}

	if err := c.postsRepo.InsertEngagementsCount(ctx, req.Msg.PostId); err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to initialize post engagements"))
	}

	return connect.NewResponse(&postsv1.InitializePostEngagementsResponse{
		True: true,
	}), nil

}
func (c *PostController) CreatePostIndexedByUser(
	ctx context.Context,
	req *connect.Request[postsv1.CreatePostIndexedByUserRequest],
) (*connect.Response[postsv1.CreatePostIndexedByUserResponse], error) {

	if req.Msg.Post == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid fields"))
	}

	if err := c.postsRepo.CreatePostIndexedByUser(ctx, req.Msg.Post); err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to index post by user"))
	}

	return connect.NewResponse(&postsv1.CreatePostIndexedByUserResponse{
		Success: true,
	}), nil

}

// TODO-remove this later
func (c *PostController) UpdatePostEngagements(ctx context.Context, req *connect.Request[postsv1.UpdatePostEngagementsRequest],
) (*connect.Response[postsv1.UpdatePostEngagementsResponse], error) {
	if req.Msg.PostId == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid fields"))
	}

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := c.postsRepo.SafeIncrementEngagementCounts(egCtx, req.Msg.PostId, "comment_count"); err != nil {
			return err
		}
		return nil

	})

	eg.Go(func() error {
		if err := c.postsRepo.SafeIncrementEngagementCounts(egCtx, req.Msg.PostId, "like_count"); err != nil {
			return err
		}
		return nil
	})
	eg.Go(func() error {
		if err := c.postsRepo.SafeIncrementEngagementCounts(egCtx, req.Msg.PostId, "share_count"); err != nil {
			return err
		}
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to increment follow counts: %w", err))
	}
	return connect.NewResponse(&postsv1.UpdatePostEngagementsResponse{
		Success: true,
	}), nil

}

func (c *PostController) CreateLike(ctx context.Context, req *connect.Request[postsv1.CreateLikeRequest]) (*connect.Response[postsv1.CreateLikeResponse], error) {
	if req.Msg.PostId == 0 || req.Msg.UserId == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid fields"))
	}

	user, err := auth.GetUserFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("unauthorized"))
	}

	if req.Msg.UserId != user.Id {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("ids dont match"))

	}

	like := &postsv1.Like{
		PostId:    req.Msg.PostId,
		UserId:    user.Id,
		CreatedAt: timestamppb.Now(),
	}
	if err := c.postsRepo.CreateLike(ctx, like); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create like: %w", err))
	}
	return connect.NewResponse(&postsv1.CreateLikeResponse{
		Like: like,
	}), nil

}

func (c *PostController) CreateLikeByUser(
	ctx context.Context,
	req *connect.Request[postsv1.CreateLikeByUserRequest],
) (*connect.Response[postsv1.CreateLikeByUserResponse], error) {

	if req.Msg.Like == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("like is required"))
	}

	if err := c.postsRepo.CreateUserLike(ctx, req.Msg.GetLike()); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create user like: %w", err))
	}

	return connect.NewResponse(&postsv1.CreateLikeByUserResponse{
		Created: true,
	}), nil
}

func (c *PostController) IncrementPostLikes(
	ctx context.Context,
	req *connect.Request[postsv1.IncrementPostLikesRequest],
) (*connect.Response[postsv1.IncrementPostLikesResponse], error) {
	if req.Msg.GetPostId() == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("post_id is required"))
	}

	if err := c.postsRepo.SafeIncrementEngagementCounts(ctx, req.Msg.GetPostId(), "like_count"); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to increment like count: %w", err))
	}

	return connect.NewResponse(&postsv1.IncrementPostLikesResponse{
		Incremented: true,
	}), nil
}

func (c *PostController) GetPostWithMetadata(ctx context.Context, req *connect.Request[postsv1.GetPostRequest]) (*connect.Response[postsv1.GetPostWithMetadataResponse], error) {
	if req.Msg.GetPostId() == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("post_id is required"))
	}

	postId := req.Msg.GetPostId()

	// Fetch the appropriate post
	post, err := c.postsRepo.GetPost(ctx, postId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("post not found: %w", err))
	}

	// Fetch engagement counts
	postEngagements, err := c.postsRepo.SelectEngagementCounts(ctx, postId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("post engagements not found: %w", err))
	}

	resp := &postsv1.GetPostWithMetadataResponse{
		Post:         post,
		LikeCount:    postEngagements.LikeCount,
		ShareCount:   postEngagements.ShareCount,
		CommentCount: postEngagements.CommentCount,
		RepostCount:  postEngagements.RepostCount,
	}

	return connect.NewResponse(resp), nil
}
