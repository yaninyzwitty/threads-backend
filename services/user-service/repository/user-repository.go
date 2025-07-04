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
	query := `INSERT INTO threads_keyspace.users (id, username, full_name, email, profile_pic_url, is_verified, created_at, updated_at, password) VALUES(?, ?, ?, ?, ?, false, ?, ?, ?)`

	return r.session.Query(query, user.Id, user.Username, user.FullName, user.Email, user.ProfilePicUrl, user.CreatedAt.AsTime(), user.UpdatedAt.AsTime(), user.Password).Exec()

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
	query := `SELECT id, username, full_name, email, profile_pic_url, is_verified, created_at, updated_at, password FROM threads_keyspace.users WHERE id = ?`
	var user userv1.User
	var createdAt, updatedAt time.Time
	err := r.session.Query(query, id).Scan(&user.Id, &user.Username, &user.FullName, &user.Email, &user.ProfilePicUrl, &user.IsVerified, &createdAt, &updatedAt, &user.Password)
	if err != nil {
		return nil, err
	}

	user.CreatedAt = timestamppb.New(createdAt)
	user.UpdatedAt = timestamppb.New(updatedAt)
	return &user, nil
}
func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (*userv1.User, error) {
	query := `SELECT id, username, full_name, email, profile_pic_url, is_verified, created_at, updated_at, password FROM threads_keyspace.users WHERE email = ?`
	var user userv1.User
	var createdAt, updatedAt time.Time
	err := r.session.Query(query, email).Scan(&user.Id, &user.Username, &user.FullName, &user.Email, &user.ProfilePicUrl, &user.IsVerified, &createdAt, &updatedAt, &user.Password)
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

func (r *UserRepository) SaveFollowRelationAndEmitEvent(ctx context.Context, userID, followingID int64, now time.Time) error {
	if userID == followingID {
		return fmt.Errorf("user cannot follow themselves")
	}

	const (
		followingQuery = `INSERT INTO threads_keyspace.following_by_user (user_id, following_id, following_at) VALUES (?, ?, ?)`
		followerQuery  = `INSERT INTO threads_keyspace.followers_by_user (user_id, follower_id, followed_at) VALUES (?, ?, ?)`
		outboxQuery    = `INSERT INTO threads_keyspace.outbox (event_id, event_type, payload, published) VALUES (uuid(), ?, ?, false) USING TTL 86400`
		eventType      = "user.followed"
	)

	batch := r.session.NewBatch(gocql.LoggedBatch).WithContext(ctx)

	// Insert into both tables
	batch.Query(followingQuery, userID, followingID, now)
	batch.Query(followerQuery, followingID, userID, now)

	// Event payload
	payload, err := protojson.Marshal(&userv1.FollowedEvent{
		UserId:      userID,
		FollowingId: followingID,
		FollowedAt:  timestamppb.New(now),
	})
	if err != nil {
		return fmt.Errorf("failed to marshal follow event: %w", err)
	}

	// Add to outbox
	batch.Query(outboxQuery, eventType, payload)

	if err := r.session.ExecuteBatch(batch); err != nil {
		return fmt.Errorf("failed to execute follow batch: %w", err)
	}

	return nil
}

func (r *UserRepository) UnfollowUser(ctx context.Context, followerID, userId int64, now time.Time) error {
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
	loggedBatch.Query(unfollowQuery, userId, followerID)

	// Marshal the event payload
	payload, err := protojson.Marshal(&userv1.UnfollowedEvent{
		UserId:       userId,
		FollowingId:  followerID,
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

func (r *UserRepository) IncrementFollowingCount(ctx context.Context, userId int64) error {
	query := `UPDATE threads_keyspace.follower_counts SET following_count = following_count + 1 WHERE user_id = ?`
	return r.session.Query(query, userId).WithContext(ctx).Exec()
}

func (r *UserRepository) IncrementFollowerCount(ctx context.Context, recipientId int64) error {
	query := `UPDATE threads_keyspace.follower_counts SET follower_count = follower_count + 1 WHERE user_id = ?`
	return r.session.Query(query, recipientId).WithContext(ctx).Exec()
}
func (r *UserRepository) DecrementFollowingCount(ctx context.Context, userId int64) error {
	query := `UPDATE threads_keyspace.follower_counts SET following_count = following_count - 1 WHERE user_id = ?`
	return r.session.Query(query, userId).WithContext(ctx).Exec()
}

func (r *UserRepository) DecrementFollowerCount(ctx context.Context, recipientId int64) error {
	query := `UPDATE threads_keyspace.follower_counts SET follower_count = follower_count - 1 WHERE user_id = ?`
	return r.session.Query(query, recipientId).WithContext(ctx).Exec()
}
