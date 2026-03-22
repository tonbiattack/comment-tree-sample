// このファイルは隣接リスト方式のコメントリポジトリを実装します。
//
// 隣接リスト方式では、各コメントが親コメントの ID（parent_id）のみを持ちます。
// path・depth カラムはこの方式では更新せず、テーブルの DEFAULT 値（"/"、0）のままとします。
// サブツリーの取得には MySQL の再帰CTE（WITH RECURSIVE）を利用します。
package commenttree

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrParentNotFound は指定した親コメントIDが存在しない場合に返されるエラーです。
// 異なる投稿に属するコメントを親として指定した場合や、存在しないIDを指定した場合に返されます。
var ErrParentNotFound = errors.New("parent comment not found")

// Repository は隣接リスト方式のコメントリポジトリです。
//
// SQL クエリをファイルから読み込む設計にすることで、
// Go コードと SQL を分離して管理しやすくしています。
type Repository struct {
	// db はデータベース接続。複数のゴルーチンから安全に共有できます。
	db *sql.DB
	// sqlRoot は SQL クエリファイルが格納されたディレクトリのパス。
	// 実行時は "sql/queries"、テスト時は testdb.SQLQueryDir() で解決されます。
	sqlRoot string
}

// NewRepository は新しい Repository を生成して返します。
//
// sqlRoot には SQL クエリファイルのディレクトリパスを指定します。
// 例: "sql/queries"
func NewRepository(db *sql.DB, sqlRoot string) *Repository {
	return &Repository{
		db:      db,
		sqlRoot: sqlRoot,
	}
}

// CreateComment は新しいコメントをデータベースに挿入します。
//
// 隣接リスト方式では path・depth は更新しません（DEFAULT 値のまま）。
// 親コメントが指定された場合は、同じ投稿内に存在するかを事前に確認します。
//
// 挿入成功後、comment.ID にデータベースで採番されたIDが設定されます。
func (r *Repository) CreateComment(ctx context.Context, comment *Comment) error {
	if comment == nil {
		return errors.New("comment is nil")
	}

	// 親コメントの存在確認（異なる投稿へのぶら下がりを防ぐため post_id も条件に含める）
	if comment.ParentID != nil {
		var count int
		if err := r.db.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM comments WHERE id = ? AND post_id = ?`,
			*comment.ParentID,
			comment.PostID,
		).Scan(&count); err != nil {
			return fmt.Errorf("check parent comment: %w", err)
		}
		if count == 0 {
			return ErrParentNotFound
		}
	}

	// コメントを挿入する。path・depth はカラムのDEFAULT値（"/"・0）が使われる。
	result, err := r.db.ExecContext(
		ctx,
		`INSERT INTO comments (post_id, parent_id, body, created_at) VALUES (?, ?, ?, ?)`,
		comment.PostID,
		comment.ParentID,
		comment.Body,
		comment.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert comment: %w", err)
	}

	// 採番されたIDを呼び出し元の構造体に書き戻す
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get inserted comment id: %w", err)
	}

	comment.ID = id
	return nil
}

// GetRootCommentsByPostID は指定した投稿のルートコメント（parent_id IS NULL）一覧を返します。
//
// SQL は root_comments.sql から読み込みます。
func (r *Repository) GetRootCommentsByPostID(ctx context.Context, postID int64) ([]Comment, error) {
	query, err := r.loadQuery("root_comments.sql")
	if err != nil {
		return nil, err
	}
	return r.queryComments(ctx, query, postID)
}

// GetCommentSubtree は指定したコメントとその全子孫を返します。
//
// SQL は comment_subtree.sql から読み込みます。
// 内部では WITH RECURSIVE を使った再帰クエリにより深さ優先で子孫を展開します。
func (r *Repository) GetCommentSubtree(ctx context.Context, commentID int64) ([]Comment, error) {
	query, err := r.loadQuery("comment_subtree.sql")
	if err != nil {
		return nil, err
	}
	return r.queryComments(ctx, query, commentID)
}

// GetPostCommentTree は指定した投稿の全コメントをツリー構造で返します。
//
// SQL は post_comment_tree.sql から読み込みます。
// データベースから全コメントをフラットに取得した後、
// メモリ上で BuildTree を呼んでツリー構造に変換します。
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

// loadQuery は sqlRoot ディレクトリから指定した名前の SQL ファイルを読み込みます。
func (r *Repository) loadQuery(name string) (string, error) {
	path := filepath.Join(r.sqlRoot, name)
	query, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read sql file %s: %w", path, err)
	}
	return string(query), nil
}

// queryComments は SQL クエリを実行してコメントのスライスを返す汎用ヘルパーです。
//
// parent_id カラムは NULL 許容のため sql.NullInt64 で受け取り、
// 有効な場合のみ Comment.ParentID ポインタに変換します。
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
			parentID sql.NullInt64 // NULL 許容カラムは NullInt64 で受け取る
		)
		if err := rows.Scan(&comment.ID, &comment.PostID, &parentID, &comment.Body, &comment.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan comment: %w", err)
		}
		// NullInt64.Valid が true の場合のみポインタに変換する（NULL = ルートコメント）
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
