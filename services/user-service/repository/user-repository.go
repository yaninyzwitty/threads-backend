package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/gocql/gocql"
	"github.com/redis/go-redis/v9"
	userv1 "github.com/yaninyzwitty/threads-go-backend/gen/user/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type UserRepository struct {
	session *gocql.Session
	cache   *redis.Client
}

func NewUserRepository(session *gocql.Session, cache *redis.Client) *UserRepository {
	return &UserRepository{
		session: session,
		cache:   cache,
	}
}

func (r *UserRepository) CreateUser(ctx context.Context, user *userv1.User) error {
	query := `
		INSERT INTO threads_keyspace.users 
		(id, username, full_name, email, profile_pic_url, is_verified, created_at, updated_at, password) 
		VALUES (?, ?, ?, ?, ?, false, ?, ?, ?)`

	return r.session.Query(query,
		user.Id, user.Username, user.FullName, user.Email, user.ProfilePicUrl,
		user.CreatedAt.AsTime(), user.UpdatedAt.AsTime(), user.Password).
		WithContext(ctx).
		Exec()
}

func (r *UserRepository) CreateUserWithInitialCounts(ctx context.Context, user *userv1.User) error {
	const (
		insertUserQuery = `
			INSERT INTO threads_keyspace.users (
				id, username, full_name, email, password, profile_pic_url,
				is_verified, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

		insertOutboxQuery = `
			INSERT INTO threads_keyspace.outbox (
				event_id, event_type, payload, published
			) VALUES (
				uuid(), ?, ?, false
			) USING TTL 86400`

		eventType = "user.created"
	)

	// Marshal the user payload for outbox
	payload, err := protojson.Marshal(user)
	if err != nil {
		return fmt.Errorf("failed to marshal user for outbox: %w", err)
	}

	// Create a logged batch for atomic execution
	batch := r.session.NewBatch(gocql.LoggedBatch).WithContext(ctx)

	// Insert user
	batch.Query(insertUserQuery, user.Id, user.Username, user.FullName, user.Email,
		user.Password, user.ProfilePicUrl, user.IsVerified, user.CreatedAt.AsTime(), user.UpdatedAt.AsTime())

	// Insert outbox event
	batch.Query(insertOutboxQuery, eventType, payload)

	// Execute the batch
	if err := r.session.ExecuteBatch(batch); err != nil {
		return fmt.Errorf("failed to execute user creation batch: %w", err)
	}

	return nil
}

func (r *UserRepository) UpdateUser(ctx context.Context, user *userv1.User) error {
	query := `
		UPDATE threads_keyspace.users 
		SET username = ?, full_name = ?, email = ?, profile_pic_url = ?, updated_at = ? 
		WHERE id = ?`

	return r.session.Query(query,
		user.Username, user.FullName, user.Email, user.ProfilePicUrl,
		user.UpdatedAt.AsTime(), user.Id).
		WithContext(ctx).
		Exec()
}

func (r *UserRepository) DeleteUser(ctx context.Context, id int64) error {
	query := `DELETE FROM threads_keyspace.users WHERE id = ?`
	return r.session.Query(query, id).WithContext(ctx).Exec()
}

func (r *UserRepository) GetUserByID(ctx context.Context, id int64) (*userv1.User, error) {
	query := `
		SELECT id, username, full_name, email, profile_pic_url, is_verified, created_at, updated_at, password 
		FROM threads_keyspace.users 
		WHERE id = ?`

	var user userv1.User
	var createdAt, updatedAt time.Time

	err := r.session.Query(query, id).WithContext(ctx).
		Scan(&user.Id, &user.Username, &user.FullName, &user.Email, &user.ProfilePicUrl,
			&user.IsVerified, &createdAt, &updatedAt, &user.Password)
	if err != nil {
		return nil, err
	}

	user.CreatedAt = timestamppb.New(createdAt)
	user.UpdatedAt = timestamppb.New(updatedAt)
	return &user, nil
}

func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (*userv1.User, error) {
	query := `
		SELECT id, username, full_name, email, profile_pic_url, is_verified, created_at, updated_at, password 
		FROM threads_keyspace.users 
		WHERE email = ?`

	var user userv1.User
	var createdAt, updatedAt time.Time

	err := r.session.Query(query, email).WithContext(ctx).
		Scan(&user.Id, &user.Username, &user.FullName, &user.Email, &user.ProfilePicUrl,
			&user.IsVerified, &createdAt, &updatedAt, &user.Password)
	if err != nil {
		return nil, err
	}

	user.CreatedAt = timestamppb.New(createdAt)
	user.UpdatedAt = timestamppb.New(updatedAt)
	return &user, nil
}

func (r *UserRepository) ListUsers(ctx context.Context, pageSize int, pagingState []byte) ([]*userv1.User, []byte, error) {
	query := `
		SELECT id, username, full_name, email, profile_pic_url, is_verified, created_at, updated_at 
		FROM threads_keyspace.users`

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

	for iter.Scan(&id, &username, &fullName, &email, &profilePicURL, &isVerified, &createdAt, &updatedAt) {
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

	isFollowing, err := r.IsFollowing(ctx, userID, followingID)
	if err != nil {
		return fmt.Errorf("failed to check if user is following: %w", err)
	}
	if isFollowing {
		return fmt.Errorf("user already following")
	}

	const (
		followingQuery = `INSERT INTO threads_keyspace.following_by_user (user_id, following_id, following_at) VALUES (?, ?, ?)`
		followerQuery  = `INSERT INTO threads_keyspace.followers_by_user (user_id, follower_id, followed_at) VALUES (?, ?, ?)`
		outboxQuery    = `INSERT INTO threads_keyspace.outbox (event_id, event_type, payload, published) VALUES (uuid(), ?, ?, false) USING TTL 86400`
		eventType      = "user.followed"
	)

	batch := r.session.NewBatch(gocql.LoggedBatch).WithContext(ctx)

	batch.Query(followingQuery, userID, followingID, now)
	batch.Query(followerQuery, followingID, userID, now)

	payload, err := protojson.Marshal(&userv1.FollowedEvent{
		UserId:      userID,
		FollowingId: followingID,
		FollowedAt:  timestamppb.New(now),
	})
	if err != nil {
		return fmt.Errorf("failed to marshal follow event: %w", err)
	}

	batch.Query(outboxQuery, eventType, payload)

	if err := r.session.ExecuteBatch(batch); err != nil {
		return fmt.Errorf("failed to execute follow batch: %w", err)
	}

	return nil
}

func (r *UserRepository) UnfollowUser(ctx context.Context, followerID, userId int64, now time.Time) error {
	const (
		unfollowFollowerQuery  = `DELETE FROM threads_keyspace.followers_by_user WHERE user_id = ? AND follower_id = ?`
		unfollowFollowingQuery = `DELETE FROM threads_keyspace.following_by_user WHERE user_id = ? AND following_id = ?`
		outboxQuery            = `INSERT INTO threads_keyspace.outbox (event_id, event_type, payload, published) VALUES (uuid(), ?, ?, false)`
		eventType              = "user.unfollowed"
	)

	batch := r.session.NewBatch(gocql.LoggedBatch).WithContext(ctx)

	batch.Query(unfollowFollowerQuery, userId, followerID)
	batch.Query(unfollowFollowingQuery, followerID, userId)

	payload, err := protojson.Marshal(&userv1.UnfollowedEvent{
		UserId:       userId,
		FollowingId:  followerID,
		UnfollowedAt: timestamppb.New(now),
	})
	if err != nil {
		return fmt.Errorf("failed to marshal unfollow event: %w", err)
	}

	batch.Query(outboxQuery, eventType, payload)

	if err := r.session.ExecuteBatch(batch); err != nil {
		return fmt.Errorf("failed to execute unfollow batch: %w", err)
	}

	return nil
}

func (r *UserRepository) SafeIncrement(ctx context.Context, userID int64, column string) error {
	var current int

	querySelect := fmt.Sprintf(`
		SELECT %s FROM threads_keyspace.follower_counts WHERE user_id = ?`, column)

	_ = r.session.Query(querySelect, userID).WithContext(ctx).Scan(&current)
	// If missing row or null, current = 0

	queryUpdate := fmt.Sprintf(`
		UPDATE threads_keyspace.follower_counts SET %s = ? WHERE user_id = ?`, column)

	return r.session.Query(queryUpdate, current+1, userID).WithContext(ctx).Exec()
}

func (r *UserRepository) SafeDecrement(ctx context.Context, userID int64, column string) error {
	var current int

	querySelect := fmt.Sprintf(`
		SELECT %s FROM threads_keyspace.follower_counts WHERE user_id = ?`, column)

	if err := r.session.Query(querySelect, userID).WithContext(ctx).Scan(&current); err != nil {
		return fmt.Errorf("fetch current %s failed: %w", column, err)
	}

	if current <= 0 {
		// Skip update to avoid negative count
		return nil
	}

	queryUpdate := fmt.Sprintf(`
		UPDATE threads_keyspace.follower_counts SET %s = ? WHERE user_id = ?`, column)

	return r.session.Query(queryUpdate, current-1, userID).WithContext(ctx).Exec()
}

func (r *UserRepository) FollowUserCached(ctx context.Context, userId, followingId int64) error {
	key := fmt.Sprintf("user:%d:following:%d", userId, followingId)
	return r.cache.Set(ctx, key, "1", 0).Err() // no expiration
}

func (r *UserRepository) IsFollowing(ctx context.Context, userId, followingId int64) (bool, error) {
	key := fmt.Sprintf("user:%d:following:%d", userId, followingId)
	val, err := r.cache.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return val == "1", nil
}

func (r *UserRepository) UnfollowUserCached(ctx context.Context, userId, followingId int64) error {
	key := fmt.Sprintf("user:%d:following:%d", userId, followingId)
	return r.cache.Del(ctx, key).Err()
}

func (r *UserRepository) InsertFollowerCount(ctx context.Context, userId int64) error {
	insertFollowerCountsQuery := `
			INSERT INTO threads_keyspace.follower_counts (
				user_id, follower_count, following_count
			) VALUES (?, 0, 0)`

	return r.session.Query(insertFollowerCountsQuery, userId).WithContext(ctx).Exec()

}
