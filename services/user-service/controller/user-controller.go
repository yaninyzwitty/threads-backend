package controller

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"connectrpc.com/connect"
	userv1 "github.com/yaninyzwitty/threads-go-backend/gen/user/v1"
	"github.com/yaninyzwitty/threads-go-backend/services/user-service/auth"
	"github.com/yaninyzwitty/threads-go-backend/services/user-service/repository"
	"github.com/yaninyzwitty/threads-go-backend/shared/snowflake"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type UserController struct {
	userRepo *repository.UserRepository
	store    auth.RefreshTokenStore
}

// NewUserController creates a new UserController with the provided user repository and refresh token store.
func NewUserController(userRepo *repository.UserRepository, store auth.RefreshTokenStore) *UserController {
	return &UserController{
		userRepo: userRepo,
		store:    store,
	}
}

// ---------------- Create User ------------------
func (c *UserController) CreateUser(
	ctx context.Context,
	req *connect.Request[userv1.CreateUserRequest],
) (*connect.Response[userv1.CreateUserResponse], error) {

	if req.Msg.Username == "" || req.Msg.Email == "" || req.Msg.FullName == "" ||
		req.Msg.Password == "" || req.Msg.ProfilePicUrl == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid request"))
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Msg.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to hash password"))
	}

	userId, err := snowflake.GenerateID()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to generate id"))
	}

	user := &userv1.User{
		Id:            int64(userId),
		Username:      req.Msg.Username,
		FullName:      req.Msg.FullName,
		Email:         req.Msg.Email,
		ProfilePicUrl: req.Msg.ProfilePicUrl,
		IsVerified:    req.Msg.IsVerified,
		Password:      string(hashedPassword),
		CreatedAt:     timestamppb.Now(),
		UpdatedAt:     timestamppb.Now(),
	}

	if err := c.userRepo.CreateUser(ctx, user); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create user: %w", err))
	}

	return connect.NewResponse(&userv1.CreateUserResponse{
		User: user,
	}), nil
}

// ---------------- Login User ------------------
func (c *UserController) LoginUser(
	ctx context.Context,
	req *connect.Request[userv1.LoginUserRequest],
) (*connect.Response[userv1.LoginUserResponse], error) {

	if req.Msg.Email == "" || req.Msg.Password == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid request"))
	}

	user, err := c.userRepo.GetUserByEmail(ctx, req.Msg.Email)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get user: %w", err))
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Msg.Password)); err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid credentials"))
	}

	accessToken, err := auth.GenerateJWT_Token(user.Id, user.Email, user.Username, user.FullName)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to generate token"))
	}

	userIdStr := strconv.FormatInt(user.Id, 10)
	refreshToken, err := c.store.CreateRefreshToken(ctx, userIdStr)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to generate refresh token"))
	}

	return connect.NewResponse(&userv1.LoginUserResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}), nil
}

// ---------------- Update User ------------------
func (c *UserController) UpdateUser(
	ctx context.Context,
	req *connect.Request[userv1.UpdateUserRequest],
) (*connect.Response[userv1.UpdateUserResponse], error) {

	if req.Msg.Id == 0 || req.Msg.Username == "" || req.Msg.Email == "" ||
		req.Msg.FullName == "" || req.Msg.ProfilePicUrl == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid request"))
	}

	user, err := c.userRepo.GetUserByID(ctx, req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get user: %w", err))
	}

	user.Username = req.Msg.Username
	user.FullName = req.Msg.FullName
	user.Email = req.Msg.Email
	user.ProfilePicUrl = req.Msg.ProfilePicUrl
	user.UpdatedAt = timestamppb.Now()

	if err := c.userRepo.UpdateUser(ctx, user); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update user: %w", err))
	}

	return connect.NewResponse(&userv1.UpdateUserResponse{
		User: user,
	}), nil
}

// ---------------- Delete User ------------------
func (c *UserController) DeleteUser(
	ctx context.Context,
	req *connect.Request[userv1.DeleteUserRequest],
) (*connect.Response[userv1.DeleteUserResponse], error) {

	if req.Msg.Id == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid request"))
	}

	if err := c.userRepo.DeleteUser(ctx, req.Msg.Id); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete user: %w", err))
	}

	return connect.NewResponse(&userv1.DeleteUserResponse{
		Success: true,
	}), nil
}

// ---------------- Get User By ID ------------------
func (c *UserController) GetUserByID(
	ctx context.Context,
	req *connect.Request[userv1.GetUserByIDRequest],
) (*connect.Response[userv1.GetUserByIDResponse], error) {

	if req.Msg.Id == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid request"))
	}

	user, err := c.userRepo.GetUserByID(ctx, req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get user: %w", err))
	}

	return connect.NewResponse(&userv1.GetUserByIDResponse{
		User: user,
	}), nil
}

// ---------------- List Users ------------------
func (c *UserController) ListUsers(
	ctx context.Context,
	req *connect.Request[userv1.ListUsersRequest],
) (*connect.Response[userv1.ListUsersResponse], error) {

	if req.Msg.PageSize <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("page size must be greater than 0"))
	}

	users, nextPageToken, err := c.userRepo.ListUsers(ctx, int(req.Msg.PageSize), req.Msg.PageToken)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list users: %w", err))
	}

	return connect.NewResponse(&userv1.ListUsersResponse{
		Users:         users,
		NextPageToken: nextPageToken,
	}), nil
}

// ---------------- Follow User ------------------
func (c *UserController) FollowUser(
	ctx context.Context,
	req *connect.Request[userv1.FollowUserRequest],
) (*connect.Response[userv1.FollowUserResponse], error) {

	if req.Msg.FollowingId == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid request"))
	}

	userFromCtx, err := auth.GetUserFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
	}

	if userFromCtx.Id == req.Msg.FollowingId {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("cannot follow yourself"))
	}

	if err := c.userRepo.FollowUser(ctx, userFromCtx.Id, req.Msg.FollowingId, time.Now()); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to follow user: %w", err))
	}

	return connect.NewResponse(&userv1.FollowUserResponse{Success: true}), nil
}

// ---------------- Unfollow User ------------------
func (c *UserController) UnfollowUser(
	ctx context.Context,
	req *connect.Request[userv1.UnfollowUserRequest],
) (*connect.Response[userv1.UnfollowUserResponse], error) {

	if req.Msg.FollowingId == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid request"))
	}

	userFromCtx, err := auth.GetUserFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
	}

	if userFromCtx.Id == req.Msg.FollowingId {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("cannot unfollow yourself"))
	}

	if err := c.userRepo.UnfollowUser(ctx, userFromCtx.Id, req.Msg.FollowingId, time.Now()); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to unfollow user: %w", err))
	}

	return connect.NewResponse(&userv1.UnfollowUserResponse{Success: true}), nil
}

// ---------------- Refresh Token ------------------
func (c *UserController) RefreshToken(
	ctx context.Context,
	req *connect.Request[userv1.RefreshTokenRequest],
) (*connect.Response[userv1.RefreshTokenResponse], error) {

	if req.Msg.RefreshToken == "" || req.Msg.UserId == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid request"))
	}

	userIdStr := strconv.FormatInt(req.Msg.UserId, 10)

	valid, err := c.store.ValidateRefreshToken(ctx, req.Msg.RefreshToken, userIdStr)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to validate refresh token: %w", err))
	}
	if !valid {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid refresh token"))
	}

	user, err := c.userRepo.GetUserByID(ctx, req.Msg.UserId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get user: %w", err))
	}

	accessToken, err := auth.GenerateJWT_Token(user.Id, user.Email, user.Username, user.FullName)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to generate access token"))
	}

	newRefreshToken, err := c.store.CreateRefreshToken(ctx, userIdStr)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to create refresh token"))
	}

	// Rotate: delete old token
	_ = c.store.DeleteRefreshToken(ctx, userIdStr, req.Msg.RefreshToken)

	return connect.NewResponse(&userv1.RefreshTokenResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
	}), nil
}
