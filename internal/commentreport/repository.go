package commentreport

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type PostDiscussionSnapshot struct {
	PostID            int64
	PostTitle         string
	RootCommentCount  int
	TotalCommentCount int
	MaxDepth          int
	LatestCommentAt   time.Time
}

type RootThreadSummary struct {
	RootCommentID    int64
	RootBody         string
	DirectReplyCount int
	DescendantCount  int
	MaxDepth         int
	LatestReplyAt    time.Time
}

type Repository struct {
	db      *sql.DB
	sqlRoot string
}

func NewRepository(db *sql.DB, sqlRoot string) *Repository {
	return &Repository{db: db, sqlRoot: sqlRoot}
}

func (r *Repository) GetPostDiscussionSnapshot(ctx context.Context, postID int64) (*PostDiscussionSnapshot, error) {
	query, err := r.loadQuery("post_discussion_snapshot.sql")
	if err != nil {
		return nil, err
	}

	var (
		snapshot PostDiscussionSnapshot
		latest   sql.NullTime
	)
	if err := r.db.QueryRowContext(ctx, query, postID).Scan(
		&snapshot.PostID,
		&snapshot.PostTitle,
		&snapshot.RootCommentCount,
		&snapshot.TotalCommentCount,
		&snapshot.MaxDepth,
		&latest,
	); err != nil {
		return nil, fmt.Errorf("query post discussion snapshot: %w", err)
	}
	if latest.Valid {
		snapshot.LatestCommentAt = latest.Time
	}
	return &snapshot, nil
}

func (r *Repository) ListRootThreadSummaries(ctx context.Context, postID int64) ([]RootThreadSummary, error) {
	query, err := r.loadQuery("root_thread_summaries.sql")
	if err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx, query, postID)
	if err != nil {
		return nil, fmt.Errorf("query root thread summaries: %w", err)
	}
	defer rows.Close()

	var summaries []RootThreadSummary
	for rows.Next() {
		var (
			summary RootThreadSummary
			latest  sql.NullTime
		)
		if err := rows.Scan(
			&summary.RootCommentID,
			&summary.RootBody,
			&summary.DirectReplyCount,
			&summary.DescendantCount,
			&summary.MaxDepth,
			&latest,
		); err != nil {
			return nil, fmt.Errorf("scan root thread summary: %w", err)
		}
		if latest.Valid {
			summary.LatestReplyAt = latest.Time
		}
		summaries = append(summaries, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate root thread summaries: %w", err)
	}

	return summaries, nil
}

func (r *Repository) loadQuery(name string) (string, error) {
	path := filepath.Join(r.sqlRoot, name)
	query, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read sql file %s: %w", path, err)
	}
	return string(query), nil
}
