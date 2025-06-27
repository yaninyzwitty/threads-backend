package repository

import (
	"context"
	"time"

	"github.com/gocql/gocql"
	userv1 "github.com/yaninyzwitty/threads-go-backend/gen/user/v1"
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
	query := `
		INSERT INTO threads_go_keyspace.users (id, username, full_name, email, profile_pic_url, tags, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	return r.session.Query(query, user.Id, user.Username, user.FullName, user.Email, user.ProfilePicUrl, user.Tags, user.CreatedAt, user.UpdatedAt).Exec()

}

func (r *UserRepository) UpdateUser(ctx context.Context, user *userv1.User) error {
	query := `UPDATE threads_go_keyspace.users SET username = ?, full_name = ?, email = ?, profile_pic_url = ?, tags = ?, updated_at = ? WHERE id = ?`
	return r.session.Query(query, user.Username, user.FullName, user.Email, user.ProfilePicUrl, user.Tags, user.UpdatedAt, user.Id).Exec()
}

func (r *UserRepository) DeleteUser(ctx context.Context, id int64) error {
	query := `DELETE FROM threads_go_keyspace.users WHERE id = ?`
	return r.session.Query(query, id).Exec()
}

func (r *UserRepository) GetUserByID(ctx context.Context, id int64) (*userv1.User, error) {
	query := `SELECT id, username, full_name, email, profile_pic_url, tags, created_at, updated_at FROM threads_go_keyspace.users WHERE id = ?`
	var user userv1.User
	var createdAt, updatedAt time.Time
	err := r.session.Query(query, id).Scan(&user.Id, &user.Username, &user.FullName, &user.Email, &user.ProfilePicUrl, &user.Tags, &createdAt, &updatedAt)
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

	query := `SELECT id, username, full_name, email, profile_pic_url, is_verified, created_at, updated_at FROM threads_go_keyspace.users`

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
			Tags:          []*userv1.UserTag{}, // Tags handled separately if needed

			CreatedAt: timestamppb.New(createdAt),
			UpdatedAt: timestamppb.New(updatedAt),
		})
	}

	nextPageState := iter.PageState()

	if err := iter.Close(); err != nil {
		return nil, nil, err
	}

	return users, nextPageState, nil
}
