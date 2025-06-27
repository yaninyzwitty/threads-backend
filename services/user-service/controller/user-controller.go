package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	userv1 "github.com/yaninyzwitty/threads-go-backend/gen/user/v1"
	userv1connect "github.com/yaninyzwitty/threads-go-backend/gen/user/v1/userv1connect"
	"github.com/yaninyzwitty/threads-go-backend/services/repository"
	"github.com/yaninyzwitty/threads-go-backend/shared/snowflake"
	"google.golang.org/protobuf/types/known/timestamppb"
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
	if req.Msg.Username == "" || req.Msg.Email == "" || req.Msg.FullName == "" || req.Msg.ProfilePicUrl == "" || len(req.Msg.Tags) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid request"))
	}

	userId, err := snowflake.GenerateID()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to generate sonyflake id"))
	}

	var userTags []*userv1.UserTag
	for _, tag := range req.Msg.Tags {
		tagId, err := snowflake.GenerateID()
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.New("failed to generate sonyflake id"))
		}
		userTags = append(userTags, &userv1.UserTag{Id: int64(tagId), Tag: tag, UserId: int64(userId)})
	}

	user := &userv1.User{
		Id:            int64(userId),
		Username:      req.Msg.Username,
		FullName:      req.Msg.FullName,
		Email:         req.Msg.Email,
		ProfilePicUrl: req.Msg.ProfilePicUrl,
		IsVerified:    false,
		Tags:          userTags,
		CreatedAt:     timestamppb.New(time.Now()),
		UpdatedAt:     timestamppb.New(time.Now()),
	}

	if err := c.userRepo.CreateUser(ctx, user); err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to create user"))
	}

	return connect.NewResponse(&userv1.CreateUserResponse{
		User: user,
	}), nil
}

func (c *UserController) UpdateUser(ctx context.Context, req *connect.Request[userv1.UpdateUserRequest]) (*connect.Response[userv1.UpdateUserResponse], error) {
	if req.Msg.Id == 0 || req.Msg.Username == "" || req.Msg.Email == "" || req.Msg.FullName == "" || req.Msg.ProfilePicUrl == "" || len(req.Msg.Tags) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid request"))
	}

	user, err := c.userRepo.GetUserByID(ctx, req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to get user"))
	}

	var userTags []*userv1.UserTag
	for _, tag := range req.Msg.Tags {
		tagId, err := snowflake.GenerateID()
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.New("failed to generate sonyflake id"))
		}
		userTags = append(userTags, &userv1.UserTag{Id: int64(tagId), Tag: tag, UserId: int64(req.Msg.Id)})
	}

	user.Username = req.Msg.Username
	user.FullName = req.Msg.FullName
	user.Email = req.Msg.Email
	user.ProfilePicUrl = req.Msg.ProfilePicUrl
	user.Tags = userTags
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
