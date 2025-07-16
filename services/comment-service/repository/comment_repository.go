package repository

import (
	"context"
	"fmt"

	"github.com/gocql/gocql"
	commentsv1 "github.com/yaninyzwitty/threads-go-backend/gen/comment/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

type CommentRepository struct {
	session *gocql.Session
}

func NewCommentRepository(session *gocql.Session) *CommentRepository {
	return &CommentRepository{
		session: session,
	}
}

func (r *CommentRepository) CreateComment(ctx context.Context, comment *commentsv1.Comment) error {
	const (
		insertOutboxQuery = `
		INSERT INTO threads_keyspace.outbox (
			event_id,
			event_type,
			payload,
			published
		) VALUES (
			uuid(), ?, ?, false
		) USING TTL 86400;
	`

		insertCommentQuery = `
		INSERT INTO threads_keyspace.comments_by_post (
			post_id,
			comment_id,
			content,
			author_id,
			created_at,
			updated_at
		) VALUES (
			?, ?, ?, ?, ?, ?
		);
	`

		eventType = "comment.created"
	)

	payload, err := protojson.Marshal(comment)
	if err != nil {
		return fmt.Errorf("failed to marshal comment for outbox: %w", err)
	}

	// Create a logged batch for atomic execution
	batch := r.session.NewBatch(gocql.LoggedBatch).WithContext(ctx)

	batch.Query(insertCommentQuery, comment.Post.Id, comment.Id, comment.Content, comment.Author.Id, comment.CreatedAt.AsTime(), comment.UpdatedAt.AsTime())

	batch.Query(insertOutboxQuery, eventType, payload)

	if err := r.session.ExecuteBatch(batch); err != nil {
		return fmt.Errorf("failed to execute comment creation batch: %w", err)
	}

	return nil

}

func (r *CommentRepository) UpsertCommentsByID(ctx context.Context, comment *commentsv1.Comment) error {

	query := `INSERT INTO threads_keyspace.comments_by_id (
    comment_id, post_id, content, author_id, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?);`

	return r.session.Query(query, comment.Id, comment.Post.Id, comment.Content, comment.Author.Id, comment.CreatedAt.AsTime(), comment.UpdatedAt.AsTime()).WithContext(ctx).Exec()

}
func (r *CommentRepository) UpsertCommentsByAuthor(ctx context.Context, comment *commentsv1.Comment) error {
	query := `INSERT INTO threads_keyspace.comments_by_author (
		author_id, created_at, comment_id, post_id, content, updated_at
	) VALUES (?, ?, ?, ?, ?, ?);`

	return r.session.Query(query,
		comment.Author.Id,
		comment.CreatedAt.AsTime(),
		comment.Id,
		comment.Post.Id,
		comment.Content,
		comment.UpdatedAt.AsTime(),
	).WithContext(ctx).Exec()
}

func (r *CommentRepository) GetComment(ctx context.Context, commentID int64) (*commentsv1.Comment, error) {
	const query = `
		SELECT post_id, comment_id, content, author_id, created_at, updated_at
		FROM threads_keyspace.comments_by_id
		WHERE comment_id = ?;
	`

	var comment commentsv1.Comment
	if err := r.session.Query(query, commentID).
		Consistency(gocql.One).
		WithContext(ctx).
		Scan(
			&comment.Post.Id,
			&comment.Id,
			&comment.Content,
			&comment.Author.Id,
			&comment.CreatedAt,
			&comment.UpdatedAt,
		); err != nil {
		if err == gocql.ErrNotFound {
			return nil, fmt.Errorf("comment with ID %d not found: %w", commentID, err)
		}
		return nil, fmt.Errorf("failed to get comment: %w", err)
	}

	return &comment, nil
}
