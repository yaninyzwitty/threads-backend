package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/gocql/gocql"
	userv1 "github.com/yaninyzwitty/threads-go-backend/gen/user/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type UserRepository struct {
	session *gocql.Session
}

func NewUserRepository(session *gocql.Session) *UserRepository {
	return &UserRepository{
		session: session,
	}
}

func (r *UserRepository) CreateUser(ctx context.Context, user *userv1.User) error {
	query := `INSERT INTO threads_keyspace.users (id, username, full_name, email, profile_pic_url, is_verified, created_at, updated_at) VALUES(?, ?, ?, ?, ?, false, ?, ?)`

	return r.session.Query(query, user.Id, user.Username, user.FullName, user.Email, user.ProfilePicUrl, user.CreatedAt.AsTime(), user.UpdatedAt.AsTime()).Exec()

}

func (r *UserRepository) UpdateUser(ctx context.Context, user *userv1.User) error {
	query := `UPDATE threads_keyspace.users SET username = ?, full_name = ?, email = ?, profile_pic_url = ?, updated_at = ? WHERE id = ?`
	return r.session.Query(query, user.Username, user.FullName, user.Email, user.ProfilePicUrl, user.UpdatedAt.AsTime(), user.Id).Exec()
}

func (r *UserRepository) DeleteUser(ctx context.Context, id int64) error {
	query := `DELETE FROM threads_keyspace.users WHERE id = ?`
	return r.session.Query(query, id).Exec()
}

func (r *UserRepository) GetUserByID(ctx context.Context, id int64) (*userv1.User, error) {
	query := `SELECT id, username, full_name, email, profile_pic_url, is_verified, created_at, updated_at FROM threads_keyspace.users WHERE id = ?`
	var user userv1.User
	var createdAt, updatedAt time.Time
	err := r.session.Query(query, id).Scan(&user.Id, &user.Username, &user.FullName, &user.Email, &user.ProfilePicUrl, &user.IsVerified, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}

	user.CreatedAt = timestamppb.New(createdAt)
	user.UpdatedAt = timestamppb.New(updatedAt)
	return &user, nil
}

func (r *UserRepository) ListUsers(
	ctx context.Context,
	pageSize int,
	pagingState []byte,
) ([]*userv1.User, []byte, error) {

	query := `SELECT id, username, full_name, email, profile_pic_url, is_verified, created_at, updated_at FROM threads_keyspace.users`

	iter := r.session.Query(query).
		WithContext(ctx).
		PageSize(pageSize).
		PageState(pagingState).
		Iter()

	var users []*userv1.User

	var (
		id            int64
		username      string
		fullName      string
		email         string
		profilePicURL string
		isVerified    bool
		createdAt     time.Time
		updatedAt     time.Time
	)

	for iter.Scan(
		&id,
		&username,
		&fullName,
		&email,
		&profilePicURL,
		&isVerified,
		&createdAt,
		&updatedAt,
	) {
		users = append(users, &userv1.User{
			Id:            id,
			Username:      username,
			FullName:      fullName,
			Email:         email,
			ProfilePicUrl: profilePicURL,
			IsVerified:    isVerified,
			CreatedAt:     timestamppb.New(createdAt),
			UpdatedAt:     timestamppb.New(updatedAt),
		})
	}

	nextPageState := iter.PageState()

	if err := iter.Close(); err != nil {
		return nil, nil, err
	}

	return users, nextPageState, nil
}

func (r *UserRepository) FollowUser(ctx context.Context, userID, followingID int64, now time.Time) error {

	if userID == followingID {
		return fmt.Errorf("user cannot follow themselves")
	}

	followUserQuery := `INSERT INTO threads_keyspace.followers_by_user (user_id, follower_id, followed_at) VALUES (?, ?, ?)`

	outboxQuery := `INSERT INTO threads_keyspace.outbox (event_id, event_type, payload, published) VALUES (uuid(), ?, ?, false)`

	// use batch to store data in both outbox table and followings table
	const evtType = "user.followed"

	// create a batch to store data in both outbox table and followings table
	loggedBatch := r.session.NewBatch(gocql.LoggedBatch)

	// attach context
	loggedBatch.WithContext(ctx)

	loggedBatch.Query(followUserQuery, userID, followingID, now) // insert to the following table

	payload, err := protojson.Marshal(&userv1.FollowedEvent{
		UserId:      userID,
		FollowingId: followingID,
		FollowedAt:  timestamppb.New(now),
	})

	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	loggedBatch.Query(outboxQuery, evtType, payload) // insert to the outbox table

	// execute the batch
	if err := r.session.ExecuteBatch(loggedBatch); err != nil {
		return fmt.Errorf("failed to execute batch: %w", err)
	}

	return nil

}

func (r *UserRepository) UnfollowUser(ctx context.Context, userID, followingID int64, now time.Time) error {
	// SQL queries
	unfollowQuery := `
		DELETE FROM threads_keyspace.followers_by_user 
		WHERE user_id = ? AND following_id = ?`

	outboxQuery := `
		INSERT INTO threads_keyspace.outbox (event_id, event_type, payload, published) 
		VALUES (uuid(), ?, ?, false)`

	const evtType = "user.unfollowed"

	// Create logged batch
	loggedBatch := r.session.NewBatch(gocql.LoggedBatch)
	loggedBatch.WithContext(ctx)

	// Add unfollow delete query
	loggedBatch.Query(unfollowQuery, userID, followingID)

	// Marshal the event payload
	payload, err := protojson.Marshal(&userv1.UnfollowedEvent{
		UserId:       userID,
		FollowingId:  followingID,
		UnfollowedAt: timestamppb.New(now),
	})
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Add outbox event
	loggedBatch.Query(outboxQuery, evtType, payload)

	// Execute the batch
	if err := r.session.ExecuteBatch(loggedBatch); err != nil {
		return fmt.Errorf("failed to execute batch: %w", err)
	}

	return nil
}
