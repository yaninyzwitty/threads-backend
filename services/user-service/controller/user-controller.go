package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	userv1 "github.com/yaninyzwitty/threads-go-backend/gen/user/v1"
	"github.com/yaninyzwitty/threads-go-backend/services/user-service/auth"
	"github.com/yaninyzwitty/threads-go-backend/services/user-service/repository"
	"github.com/yaninyzwitty/threads-go-backend/shared/snowflake"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type UserController struct {
	userRepo *repository.UserRepository
	store    auth.RefreshTokenStore
}

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

	accessToken, err := auth.GenerateJWTToken(user.Id, user.Email, user.Username, user.FullName)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to generate token"))
	}

	refreshToken, err := c.store.CreateOrGetRefreshToken(ctx, user.Id)
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

	user, err := auth.GetUserFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
	}

	if user.Id != req.Msg.Id {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("unauthorized deletion"))
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

	// Validate target user
	if req.Msg.FollowingId == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid request"))
	}

	// Get authenticated user
	userFromCtx, err := auth.GetUserFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
	}

	if userFromCtx.Id == req.Msg.FollowingId {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("cannot follow yourself"))
	}

	// Current timestamp
	now := time.Now()

	if err := c.userRepo.SaveFollowRelationAndEmitEvent(ctx, userFromCtx.Id, req.Msg.FollowingId, now); err != nil {
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

	valid, err := c.store.ValidateRefreshToken(ctx, req.Msg.UserId, req.Msg.RefreshToken)
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

	accessToken, err := auth.GenerateJWTToken(user.Id, user.Email, user.Username, user.FullName)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to generate access token"))
	}
	// Rotate: delete old token
	if err := c.store.DeleteRefreshToken(ctx, req.Msg.UserId); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete old refresh token: %w", err))
	}

	newRefreshToken, err := c.store.CreateOrGetRefreshToken(ctx, req.Msg.UserId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to create refresh token"))
	}

	return connect.NewResponse(&userv1.RefreshTokenResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
	}), nil
}

func (c *UserController) IncrementFollowingAndFollowerCount(
	ctx context.Context,
	req *connect.Request[userv1.IncrementFollowingAndFollowerCountRequest],
) (*connect.Response[userv1.IncrementFollowingAndFollowerCountResponse], error) {
	event := req.Msg.GetFollowedEvent()

	if event == nil || event.FollowedAt == nil || event.UserId == 0 || event.FollowingId == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("followed event is missing or invalid"))
	}

	g, ctx := errgroup.WithContext(ctx)

	// Increment following count (for the user performing the follow)
	g.Go(func() error {
		if err := c.userRepo.IncrementFollowingCount(ctx, event.UserId); err != nil {
			return fmt.Errorf("failed to increment following count: %w", err)
		}
		return nil
	})

	// Increment follower count (for the user being followed)
	g.Go(func() error {
		if err := c.userRepo.IncrementFollowerCount(ctx, event.FollowingId); err != nil {
			return fmt.Errorf("failed to increment follower count: %w", err)
		}
		return nil
	})

	// Wait for both goroutines
	if err := g.Wait(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&userv1.IncrementFollowingAndFollowerCountResponse{
		Incremented: true,
	}), nil

}
func (c *UserController) DecrementFollowingAndFollowerCount(ctx context.Context,
	req *connect.Request[userv1.DecrementFollowingAndFollowerCountRequest],
) (*connect.Response[userv1.DecrementFollowingAndFollowerCountResponse], error) {
	event := req.Msg.GetUnfollowedEvent()

	if event == nil || event.UnfollowedAt == nil || event.UserId == 0 || event.FollowingId == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("unfollowed event is missing or invalid"))
	}
	g, ctx := errgroup.WithContext(ctx)

	// Decrement following count (for the user performing the unfollow)
	g.Go(func() error {
		if err := c.userRepo.DecrementFollowingCount(ctx, event.UserId); err != nil {
			return fmt.Errorf("failed to decrement following count: %w", err)
		}
		return nil
	})

	// Decrement follower count (for the user being unfollowed)
	g.Go(func() error {
		if err := c.userRepo.DecrementFollowerCount(ctx, event.FollowingId); err != nil {
			return fmt.Errorf("failed to decrement follower count: %w", err)
		}
		return nil
	})

	// Wait for both goroutines
	if err := g.Wait(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&userv1.DecrementFollowingAndFollowerCountResponse{
		Decremented: true,
	}), nil

}
