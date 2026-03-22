// Package commentreport は、コメントに関するビジネスレポートを提供するパッケージです。
//
// このパッケージは特定のツリー実装方式に依存せず、
// コメントデータを集計・分析するクエリを担当します。
// 主な用途:
//   - 投稿ごとのコメント統計（件数・深さ・最終活動日時）
//   - ルートスレッドごとの返信状況サマリ
//   - 未返信スレッドの一覧取得
//   - 最近アクティブな投稿の一覧取得
package commentreport

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// PostDiscussionSnapshot は、ある投稿のコメント全体に関する統計スナップショットです。
//
// この情報は投稿一覧ページの「コメント○件、最終更新○○」といった
// サマリ表示に使用することを想定しています。
type PostDiscussionSnapshot struct {
	// PostID は対象投稿のID
	PostID int64
	// PostTitle は対象投稿のタイトル
	PostTitle string
	// RootCommentCount はルートコメント（parent_id IS NULL）の件数
	RootCommentCount int
	// TotalCommentCount は返信を含む全コメントの件数
	TotalCommentCount int
	// MaxDepth はコメントツリーの最大深さ（ルートコメントは depth=0）
	MaxDepth int
	// LatestCommentAt は最後にコメントが投稿された日時。
	// コメントが一件もない場合はゼロ値になります。
	LatestCommentAt time.Time
}

// RootThreadSummary は、1件のルートコメント（スレッド）に関するサマリです。
//
// スレッド一覧ページで「このトピックに○件の返信・最終更新○○」などを
// 表示するために使用することを想定しています。
type RootThreadSummary struct {
	// RootCommentID はルートコメントのID
	RootCommentID int64
	// RootBody はルートコメントの本文
	RootBody string
	// DirectReplyCount はルートコメントへの直属返信（depth=1）の件数
	DirectReplyCount int
	// DescendantCount はルートコメントの全子孫コメント数（直属返信を含む）
	DescendantCount int
	// MaxDepth はこのスレッド内の最大深さ
	MaxDepth int
	// LatestReplyAt はこのスレッドで最後に返信された日時。
	// 返信がない場合はゼロ値になります。
	LatestReplyAt time.Time
}

// PostActivitySummary は、投稿を最近のアクティビティ順に並べるための軽量なサマリです。
//
// 投稿一覧ページで「最近コメントがあった投稿順」に並べる場合に使用します。
type PostActivitySummary struct {
	// PostID は投稿のID
	PostID int64
	// PostTitle は投稿のタイトル
	PostTitle string
	// TotalCommentCount は全コメント件数
	TotalCommentCount int
	// LatestCommentAt は最後にコメントが投稿された日時。
	// コメントが一件もない場合はゼロ値になります。
	LatestCommentAt time.Time
}

// Repository はコメントレポートのリポジトリです。
//
// SQL クエリをファイルから読み込む設計にすることで、
// Go コードと SQL を分離して管理しやすくしています。
type Repository struct {
	// db はデータベース接続。複数のゴルーチンから安全に共有できます。
	db *sql.DB
	// sqlRoot は SQL クエリファイルが格納されたディレクトリのパス。
	// 実行時は "sql/queries/business"、テスト時は testdb.SQLQuerySubDir(t, "business") で解決されます。
	sqlRoot string
}

// NewRepository は新しい Repository を生成して返します。
func NewRepository(db *sql.DB, sqlRoot string) *Repository {
	return &Repository{db: db, sqlRoot: sqlRoot}
}

// GetPostDiscussionSnapshot は指定した投稿のコメント統計スナップショットを返します。
//
// SQL は post_discussion_snapshot.sql から読み込みます。
// コメントが存在しない投稿でも NULL 安全に処理し、ゼロ値を返します。
func (r *Repository) GetPostDiscussionSnapshot(ctx context.Context, postID int64) (*PostDiscussionSnapshot, error) {
	query, err := r.loadQuery("post_discussion_snapshot.sql")
	if err != nil {
		return nil, err
	}

	var (
		snapshot PostDiscussionSnapshot
		// LatestCommentAt は NULL 許容（コメントがない投稿では NULL になる）
		latest sql.NullTime
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
	// NullTime が有効な場合のみ LatestCommentAt に設定する
	if latest.Valid {
		snapshot.LatestCommentAt = latest.Time
	}
	return &snapshot, nil
}

// ListRootThreadSummaries は指定した投稿の全ルートスレッドのサマリを返します。
//
// SQL は root_thread_summaries.sql から読み込みます。
// 各ルートコメントに対して直属返信数・全子孫数・最大深さ・最終返信日時を集計します。
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
			// LatestReplyAt は NULL 許容（返信がないルートスレッドでは NULL）
			latest sql.NullTime
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

// ListUnansweredRootThreads は指定した投稿の中で直属返信が0件のルートスレッドを返します。
//
// SQL は unanswered_root_threads.sql から読み込みます。
// 「未返信スレッド通知」や「返信が必要なスレッド一覧」などに使用できます。
func (r *Repository) ListUnansweredRootThreads(ctx context.Context, postID int64) ([]RootThreadSummary, error) {
	query, err := r.loadQuery("unanswered_root_threads.sql")
	if err != nil {
		return nil, err
	}
	return r.queryRootThreadSummaries(ctx, query, postID)
}

// ListPostsByRecentActivity は全投稿を最新コメント日時の降順で返します。
//
// SQL は posts_recent_activity.sql から読み込みます。
// コメントが一件もない投稿は LatestCommentAt がゼロ値になり、リストの末尾に並びます。
func (r *Repository) ListPostsByRecentActivity(ctx context.Context) ([]PostActivitySummary, error) {
	query, err := r.loadQuery("posts_recent_activity.sql")
	if err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query posts by recent activity: %w", err)
	}
	defer rows.Close()

	var posts []PostActivitySummary
	for rows.Next() {
		var (
			summary PostActivitySummary
			// LatestCommentAt は NULL 許容（コメントがない投稿では NULL）
			latest sql.NullTime
		)
		if err := rows.Scan(
			&summary.PostID,
			&summary.PostTitle,
			&summary.TotalCommentCount,
			&latest,
		); err != nil {
			return nil, fmt.Errorf("scan post activity summary: %w", err)
		}
		if latest.Valid {
			summary.LatestCommentAt = latest.Time
		}
		posts = append(posts, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate posts by recent activity: %w", err)
	}

	return posts, nil
}

// loadQuery は sqlRoot ディレクトリから指定した名前の SQL ファイルを読み込みます。
func (r *Repository) loadQuery(name string) (string, error) {
	path := filepath.Join(r.sqlRoot, name)
	query, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read sql file %s: %w", path, err)
	}
	return string(query), nil
}

// queryRootThreadSummaries は SQL クエリを実行して RootThreadSummary のスライスを返す汎用ヘルパーです。
//
// ListUnansweredRootThreads と ListRootThreadSummaries で同じスキャンロジックを共有します。
func (r *Repository) queryRootThreadSummaries(ctx context.Context, query string, args ...any) ([]RootThreadSummary, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
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
