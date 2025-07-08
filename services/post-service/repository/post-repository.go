package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/gocql/gocql"
	postv1 "github.com/yaninyzwitty/threads-go-backend/gen/posts/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type PostRepository struct {
	session *gocql.Session
}

func NewPostRepository(session *gocql.Session) *PostRepository {
	return &PostRepository{session: session}
}

func (r *PostRepository) CreatePost(ctx context.Context, post *postv1.Post) error {

	const (
		insertPostQuery = `INSERT INTO threads_keyspace.posts (post_id, user_id, content, image_url, created_at) VALUES (?, ?, ?, ?)`

		insertOutboxQuery = `INSERT INTO threads_keyspace.outbox (id, event_type, payload, created_at) VALUES (uuid(), ?, ?, false)`

		eventType = "post.created"
	)

	// marshal the post payload for outbox

	payload, err := protojson.Marshal(post)
	if err != nil {
		return fmt.Errorf("failed to marshal post for outbox: %w", err)
	}

	// Create a logged batch for atomic execution
	batch := r.session.NewBatch(gocql.LoggedBatch).WithContext(ctx)

	// insert post

	batch.Query(insertPostQuery, post.Id, post.UserId, post.Content, post.ImageUrl, post.CreatedAt.AsTime())

	// insert outbox event
	batch.Query(insertOutboxQuery, eventType, payload)

	// execute batch
	if err := r.session.ExecuteBatch(batch); err != nil {
		return fmt.Errorf("failed to execute post creation batch: %w", err)
	}
	return nil

}

func (r *PostRepository) GetPost(ctx context.Context, postId int64) (*postv1.Post, error) {
	query := `
		SELECT post_id, user_id, content, image_url, created_at 
		FROM threads_keyspace.posts 
		WHERE post_id = ? 
		LIMIT 1
	`

	var (
		post      postv1.Post
		createdAt time.Time
	)

	err := r.session.
		Query(query, postId).
		WithContext(ctx).
		Consistency(gocql.One).
		Scan(&post.Id, &post.UserId, &post.Content, &post.ImageUrl, &createdAt)

	if err != nil {
		if err == gocql.ErrNotFound {
			return nil, fmt.Errorf("failed to find post")
		}
		return nil, err
	}

	post.CreatedAt = timestamppb.New(createdAt)

	return &post, nil
}

func (r *PostRepository) DeletePost(ctx context.Context, postId int64) error {
	query := `DELETE FROM threads_keyspace.posts WHERE post_id = ?`
	return r.session.Query(query, postId).WithContext(ctx).Exec()
}

func (r *PostRepository) ListPostsByUser(
	ctx context.Context,
	userId int64,
	pageSize int32,
	pagingState []byte,
) (*postv1.ListPostsByUserResponse, error) {
	query := `SELECT post_id, user_id, content, image_url, created_at FROM threads_keyspace.posts_by_user WHERE user_id = ?`

	q := r.session.
		Query(query, userId).
		WithContext(ctx).
		PageSize(int(pageSize)).
		Consistency(gocql.One)

	if len(pagingState) > 0 {
		q = q.PageState(pagingState)
	}

	iter := q.Iter()
	defer iter.Close()

	var (
		posts     []*postv1.Post
		postID    int64
		uid       int64
		content   string
		imageURL  string
		createdAt time.Time
	)

	for iter.Scan(&postID, &uid, &content, &imageURL, &createdAt) {
		post := &postv1.Post{
			Id:        postID,
			UserId:    uid,
			Content:   content,
			ImageUrl:  imageURL,
			CreatedAt: timestamppb.New(createdAt),
		}
		posts = append(posts, post)
	}

	if err := iter.Close(); err != nil {
		return nil, err
	}

	return &postv1.ListPostsByUserResponse{
		Posts:       posts,
		PagingState: iter.PageState(), // nil if no more pages
	}, nil
}
