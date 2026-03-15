package commenttree

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var ErrParentNotFound = errors.New("parent comment not found")

type Repository struct {
	db      *sql.DB
	sqlRoot string
}

func NewRepository(db *sql.DB, sqlRoot string) *Repository {
	return &Repository{
		db:      db,
		sqlRoot: sqlRoot,
	}
}

func (r *Repository) CreateComment(ctx context.Context, comment *Comment) error {
	if comment == nil {
		return errors.New("comment is nil")
	}

	comment.Path = "/"
	comment.Depth = 0
	if comment.ParentID != nil {
		var (
			count       int
			parentPath  string
			parentDepth int
		)
		if err := r.db.QueryRowContext(
			ctx,
			`SELECT COUNT(*), COALESCE(MAX(path), ''), COALESCE(MAX(depth), 0) FROM comments WHERE id = ? AND post_id = ?`,
			*comment.ParentID,
			comment.PostID,
		).Scan(&count, &parentPath, &parentDepth); err != nil {
			return fmt.Errorf("check parent comment: %w", err)
		}
		if count == 0 {
			return ErrParentNotFound
		}
		comment.Path = parentPath
		comment.Depth = parentDepth + 1
	}

	result, err := r.db.ExecContext(
		ctx,
		`INSERT INTO comments (post_id, parent_id, path, depth, body, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		comment.PostID,
		comment.ParentID,
		comment.Path,
		comment.Depth,
		comment.Body,
		comment.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert comment: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get inserted comment id: %w", err)
	}

	comment.ID = id
	comment.Path = fmt.Sprintf("%s%d/", comment.Path, comment.ID)
	if _, err := r.db.ExecContext(
		ctx,
		`UPDATE comments SET path = ? WHERE id = ?`,
		comment.Path,
		comment.ID,
	); err != nil {
		return fmt.Errorf("update comment path: %w", err)
	}
	return nil
}

func (r *Repository) GetRootCommentsByPostID(ctx context.Context, postID int64) ([]Comment, error) {
	query, err := r.loadQuery("root_comments.sql")
	if err != nil {
		return nil, err
	}
	return r.queryComments(ctx, query, postID)
}

func (r *Repository) GetCommentSubtree(ctx context.Context, commentID int64) ([]Comment, error) {
	query, err := r.loadQuery("comment_subtree.sql")
	if err != nil {
		return nil, err
	}
	return r.queryComments(ctx, query, commentID)
}

func (r *Repository) GetPostCommentTree(ctx context.Context, postID int64) ([]*CommentNode, error) {
	query, err := r.loadQuery("post_comment_tree.sql")
	if err != nil {
		return nil, err
	}

	comments, err := r.queryComments(ctx, query, postID)
	if err != nil {
		return nil, err
	}

	return BuildTree(comments), nil
}

func (r *Repository) loadQuery(name string) (string, error) {
	path := filepath.Join(r.sqlRoot, name)
	query, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read sql file %s: %w", path, err)
	}
	return string(query), nil
}

func (r *Repository) queryComments(ctx context.Context, query string, args ...any) ([]Comment, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query comments: %w", err)
	}
	defer rows.Close()

	var comments []Comment
	for rows.Next() {
		var (
			comment  Comment
			parentID sql.NullInt64
		)
		if err := rows.Scan(&comment.ID, &comment.PostID, &parentID, &comment.Body, &comment.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan comment: %w", err)
		}
		if parentID.Valid {
			comment.ParentID = &parentID.Int64
		}
		comments = append(comments, comment)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate comments: %w", err)
	}

	return comments, nil
}
