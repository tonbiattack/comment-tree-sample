package closuretree

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"private-comment-tree-sample/internal/commenttree"
)

var ErrParentNotFound = errors.New("parent comment not found")

type Comment = commenttree.Comment
type CommentNode = commenttree.CommentNode

type Repository struct {
	db      *sql.DB
	sqlRoot string
}

func NewRepository(db *sql.DB, sqlRoot string) *Repository {
	return &Repository{db: db, sqlRoot: sqlRoot}
}

func (r *Repository) CreateComment(ctx context.Context, comment *Comment) error {
	if comment == nil {
		return errors.New("comment is nil")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	pathPrefix := "/"
	depth := 0
	if comment.ParentID != nil {
		var (
			count       int
			parentPath  string
			parentDepth int
		)
		if err := tx.QueryRowContext(
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
		pathPrefix = parentPath
		depth = parentDepth + 1
	}

	result, err := tx.ExecContext(
		ctx,
		`INSERT INTO comments (post_id, parent_id, path, depth, body, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		comment.PostID,
		comment.ParentID,
		pathPrefix,
		depth,
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
	comment.Depth = depth
	comment.Path = fmt.Sprintf("%s%d/", pathPrefix, comment.ID)

	if _, err := tx.ExecContext(
		ctx,
		`UPDATE comments SET path = ? WHERE id = ?`,
		comment.Path,
		comment.ID,
	); err != nil {
		return fmt.Errorf("update comment path: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO comment_closures (ancestor_id, descendant_id, depth) VALUES (?, ?, 0)`,
		comment.ID,
		comment.ID,
	); err != nil {
		return fmt.Errorf("insert self closure: %w", err)
	}

	if comment.ParentID != nil {
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO comment_closures (ancestor_id, descendant_id, depth)
			 SELECT ancestor_id, ?, depth + 1
			 FROM comment_closures
			 WHERE descendant_id = ?`,
			comment.ID,
			*comment.ParentID,
		); err != nil {
			return fmt.Errorf("insert inherited closures: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

func (r *Repository) GetCommentSubtree(ctx context.Context, commentID int64) ([]Comment, error) {
	query, err := r.loadQuery("comment_subtree.sql")
	if err != nil {
		return nil, err
	}
	return r.queryComments(ctx, query, commentID)
}

func (r *Repository) GetAncestorChain(ctx context.Context, commentID int64) ([]Comment, error) {
	query, err := r.loadQuery("ancestor_chain.sql")
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
	return commenttree.BuildTree(comments), nil
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
