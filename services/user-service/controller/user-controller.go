package controller

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	userv1 "github.com/yaninyzwitty/threads-go-backend/gen/user/v1"
	userv1connect "github.com/yaninyzwitty/threads-go-backend/gen/user/v1/userv1connect"
	"github.com/yaninyzwitty/threads-go-backend/services/user-service/repository"
	"github.com/yaninyzwitty/threads-go-backend/shared/helpers"
	"github.com/yaninyzwitty/threads-go-backend/shared/snowflake"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	USER_ROLE = "user"
)

type UserController struct {
	userv1connect.UserServiceClient
	userRepo *repository.UserRepository
}

func NewUserController(userRepo *repository.UserRepository) *UserController {
	return &UserController{
		userRepo: userRepo,
	}
}

func (c *UserController) CreateUser(ctx context.Context, req *connect.Request[userv1.CreateUserRequest]) (*connect.Response[userv1.CreateUserResponse], error) {
	if req.Msg.Username == "" || req.Msg.Email == "" || req.Msg.FullName == "" || req.Msg.ProfilePicUrl == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid request"))
	}

	userId, err := snowflake.GenerateID()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to generate sonyflake id"))
	}

	user := &userv1.User{
		Id:            int64(userId),
		Username:      req.Msg.Username,
		FullName:      req.Msg.FullName,
		Email:         req.Msg.Email,
		ProfilePicUrl: req.Msg.ProfilePicUrl,
		IsVerified:    false,
		CreatedAt:     timestamppb.New(time.Now()),
		UpdatedAt:     timestamppb.New(time.Now()),
	}

	if err := c.userRepo.CreateUser(ctx, user); err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to create user"))
	}

	// generate JWT token

	token, err := helpers.GenerateToken(user.Email, USER_ROLE, user.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to generate token"))
	}

	return connect.NewResponse(&userv1.CreateUserResponse{
		User:  user,
		Token: token,
	}), nil
}

func (c *UserController) UpdateUser(ctx context.Context, req *connect.Request[userv1.UpdateUserRequest]) (*connect.Response[userv1.UpdateUserResponse], error) {
	if req.Msg.Id == 0 || req.Msg.Username == "" || req.Msg.Email == "" || req.Msg.FullName == "" || req.Msg.ProfilePicUrl == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid request"))
	}

	user, err := c.userRepo.GetUserByID(ctx, req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to get user"))
	}

	user.Username = req.Msg.Username
	user.FullName = req.Msg.FullName
	user.Email = req.Msg.Email
	user.ProfilePicUrl = req.Msg.ProfilePicUrl
	user.UpdatedAt = timestamppb.New(time.Now())

	if err := c.userRepo.UpdateUser(ctx, user); err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to update user"))
	}

	return connect.NewResponse(&userv1.UpdateUserResponse{
		User: user,
	}), nil
}

func (c *UserController) DeleteUser(ctx context.Context, req *connect.Request[userv1.DeleteUserRequest]) (*connect.Response[userv1.DeleteUserResponse], error) {
	if req.Msg.Id == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid request"))
	}

	// blacklist token to force logout after delete
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

	// blacklist token first to ensure it's invalidated even if user deletion fails
	if err := helpers.BlacklistToken(ctx, token, time.Now().Add(24*time.Hour)); err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to blacklist token"))
	}

	if err := c.userRepo.DeleteUser(ctx, req.Msg.Id); err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to delete user"))
	}

	return connect.NewResponse(&userv1.DeleteUserResponse{
		Success: true,
	}), nil
}

func (c *UserController) GetUserByID(ctx context.Context, req *connect.Request[userv1.GetUserByIDRequest]) (*connect.Response[userv1.GetUserByIDResponse], error) {
	if req.Msg.Id == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid request"))
	}

	user, err := c.userRepo.GetUserByID(ctx, req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to get user"))
	}

	return connect.NewResponse(&userv1.GetUserByIDResponse{
		User: user,
	}), nil
}

func (c *UserController) ListUsers(ctx context.Context, req *connect.Request[userv1.ListUsersRequest]) (*connect.Response[userv1.ListUsersResponse], error) {
	if req.Msg.PageSize <= 0 {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("page size must be greater than 0"),
		)
	}
	users, nextPageToken, err := c.userRepo.ListUsers(
		ctx,
		int(req.Msg.PageSize),
		req.Msg.PageToken,
	)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInternal,
			fmt.Errorf("failed to list users: %w", err),
		)
	}

	return connect.NewResponse(&userv1.ListUsersResponse{
		Users:         users,
		NextPageToken: nextPageToken,
	}), nil
}

func (c *UserController) FollowUser(ctx context.Context, req *connect.Request[userv1.FollowUserRequest]) (*connect.Response[userv1.FollowUserResponse], error) {
	if req.Msg.UserId == 0 || req.Msg.FollowingId == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid request"))
	}

	now := time.Now()

	if err := c.userRepo.FollowUser(ctx, req.Msg.UserId, req.Msg.FollowingId, now); err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to follow user"))
	}

	return connect.NewResponse(&userv1.FollowUserResponse{
		Success: true,
	}), nil
}

func (c *UserController) UnfollowUser(ctx context.Context, req *connect.Request[userv1.UnfollowUserRequest]) (*connect.Response[userv1.UnfollowUserResponse], error) {
	if req.Msg.UserId == 0 || req.Msg.FollowingId == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid request"))
	}

	now := time.Now()

	if err := c.userRepo.UnfollowUser(ctx, req.Msg.UserId, req.Msg.FollowingId, now); err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to unfollow user"))
	}

	return connect.NewResponse(&userv1.UnfollowUserResponse{
		Success: true,
	}), nil
}
