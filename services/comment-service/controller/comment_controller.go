package controller

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"
	commentsv1 "github.com/yaninyzwitty/threads-go-backend/gen/comment/v1"
	postsv1 "github.com/yaninyzwitty/threads-go-backend/gen/posts/v1"
	userv1 "github.com/yaninyzwitty/threads-go-backend/gen/user/v1"
	"github.com/yaninyzwitty/threads-go-backend/services/comment-service/repository"
	"github.com/yaninyzwitty/threads-go-backend/services/user-service/auth"
	"github.com/yaninyzwitty/threads-go-backend/shared/snowflake"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type CommentController struct {
	repository *repository.CommentRepository
}

func NewCommentController(repo *repository.CommentRepository) *CommentController {
	return &CommentController{
		repository: repo,
	}
}

func (c *CommentController) CreateComment(ctx context.Context, req *connect.Request[commentsv1.CreateCommentRequest]) (*connect.Response[commentsv1.CreateCommentResponse], error) {
	if req.Msg.GetPostId() == 0 || req.Msg.GetContent() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("author id , post id and content are missing"))
	}

	user, err := auth.GetUserFromContext(ctx)

	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
	}

	commentId, err := snowflake.GenerateID()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to generate comment id"))
	}

	comment := &commentsv1.Comment{
		Id:      int64(commentId),
		Content: req.Msg.GetContent(),
		Author: &userv1.User{
			Id: user.Id,
		},
		CreatedAt: timestamppb.Now(),
		UpdatedAt: timestamppb.Now(),
		Post: &postsv1.Post{
			Id: req.Msg.GetPostId(),
		},
	}

	if err := c.repository.CreateComment(ctx, comment); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create comment: %w", err))
	}

	return connect.NewResponse(&commentsv1.CreateCommentResponse{
		Comment: comment,
	}), nil

}

func (c *CommentController) GetComment(ctx context.Context, req *connect.Request[commentsv1.GetCommentRequest]) (*connect.Response[commentsv1.GetCommentResponse], error) {
	if req.Msg.GetId() == 0 {

		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("comment id is missing"))

	}

	comment, err := c.repository.GetComment(ctx, req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get comment: %w", err))
	}

	return connect.NewResponse(&commentsv1.GetCommentResponse{
		Comment: comment,
	}), nil
}

func (c *CommentController) HandleCommentsUpsert(
	ctx context.Context,
	req *connect.Request[commentsv1.HandleCommentsUpsertRequest],
) (*connect.Response[commentsv1.HandleCommentsUpsertResponse], error) {

	if req.Msg.Comment == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("comment can't be nil"))
	}

	eg, egCtx := errgroup.WithContext(ctx)

	// Upsert into comments_by_id
	eg.Go(func() error {
		slog.Info("upserting into comments_by_id db table")
		if err := c.repository.UpsertCommentsByID(egCtx, req.Msg.Comment); err != nil {
			return fmt.Errorf("failed to upsert to comments_by_id: %w", err)
		}
		return nil
	})

	// Upsert into comments_by_author
	eg.Go(func() error {
		slog.Info("upserting into comments_by_author table")
		if err := c.repository.UpsertCommentsByAuthor(egCtx, req.Msg.Comment); err != nil {
			return fmt.Errorf("failed to upsert to comments_by_author: %w", err)
		}
		return nil
	})

	// Wait for both upserts
	if err := eg.Wait(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &commentsv1.HandleCommentsUpsertResponse{
		Upserted: true,
	}
	return connect.NewResponse(resp), nil
}
