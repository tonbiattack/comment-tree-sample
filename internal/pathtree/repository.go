// Package pathtree は、Materialized Path 方式でコメントツリーを管理するパッケージです。
//
// Materialized Path 方式の特徴:
//   - 各コメントがルートから自分自身までの ID パスを文字列として保持する
//   - 例: コメント4（親=2、祖父=1）の path は "/1/2/4/"
//   - サブツリー取得は LIKE 'path%' で完結し、再帰クエリが不要
//   - 祖先チェーン取得はパス文字列を解析して各 ID を検索する
//   - 書き込みは INSERT → LastInsertId → UPDATE path の 2 ステップが必要
//   - 深いツリーでは path 文字列が長くなるため VARCHAR(1024) を上限として設定
package pathtree

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"private-comment-tree-sample/internal/commenttree"
)

// ErrParentNotFound は指定した親コメントIDが存在しない場合に返されるエラーです。
var ErrParentNotFound = errors.New("parent comment not found")

// commenttree パッケージの型をこのパッケージのエイリアスとして公開します。
// 呼び出し元が pathtree.Comment として使えるようにするための型エイリアスです。
type Comment = commenttree.Comment
type CommentNode = commenttree.CommentNode

// Repository は Materialized Path 方式のコメントリポジトリです。
//
// SQL クエリをファイルから読み込む設計にすることで、
// Go コードと SQL を分離して管理しやすくしています。
type Repository struct {
	// db はデータベース接続。複数のゴルーチンから安全に共有できます。
	db *sql.DB
	// sqlRoot は SQL クエリファイルが格納されたディレクトリのパス。
	// 実行時は "sql/queries/path"、テスト時は testdb.SQLQuerySubDir(t, "path") で解決されます。
	sqlRoot string
}

// NewRepository は新しい Repository を生成して返します。
func NewRepository(db *sql.DB, sqlRoot string) *Repository {
	return &Repository{db: db, sqlRoot: sqlRoot}
}

// CreateComment は新しいコメントをデータベースに挿入し、path と depth を更新します。
//
// Materialized Path 方式では以下の流れで書き込みます:
//  1. 親コメントの path と depth を取得する（ルートの場合はデフォルト値を使用）
//  2. comments テーブルにコメントを INSERT する（path はまだ仮の値）
//  3. 採番された ID を取得し、確定した path を UPDATE する
//     path の形式: "親のpath + 自分のID + /" → 例: "/1/2/" + "4" + "/" = "/1/2/4/"
//
// 閉包テーブル方式と異なりトランザクション不要（INSERT と UPDATE の 2 ステップだが、
// 中間状態を他のクエリが読まない限り問題ない）。
//
// 挿入成功後、comment.ID / comment.Depth / comment.Path が設定されます。
func (r *Repository) CreateComment(ctx context.Context, comment *Comment) error {
	if comment == nil {
		return errors.New("comment is nil")
	}

	// 親コメントの情報取得と存在チェック
	// ルートコメントの場合はデフォルト値（path="/"、depth=0）を使う
	pathPrefix := "/"
	depth := 0
	if comment.ParentID != nil {
		var (
			parentPath  string
			parentDepth int
		)
		// ErrNoRows = 親コメントが存在しない → ErrParentNotFound に変換する
		if err := r.db.QueryRowContext(
			ctx,
			`SELECT path, depth FROM comments WHERE id = ? AND post_id = ?`,
			*comment.ParentID,
			comment.PostID,
		).Scan(&parentPath, &parentDepth); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrParentNotFound
			}
			return fmt.Errorf("get parent path: %w", err)
		}
		pathPrefix = parentPath
		depth = parentDepth + 1
	}

	// コメントを INSERT する（path はまだ仮の値で後から UPDATE する）
	result, err := r.db.ExecContext(
		ctx,
		`INSERT INTO comments (post_id, parent_id, path, depth, body, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		comment.PostID,
		comment.ParentID,
		pathPrefix, // 後ほど採番 ID を含めた正式な path に UPDATE する
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
	// path は親のパスに自分の ID を付加した形式: "/parent_ids/self_id/"
	comment.Path = fmt.Sprintf("%s%d/", pathPrefix, id)

	// 確定した path で UPDATE する（INSERT 時は ID が不明なため 2 ステップになる）
	if _, err := r.db.ExecContext(ctx, `UPDATE comments SET path = ? WHERE id = ?`, comment.Path, comment.ID); err != nil {
		return fmt.Errorf("update comment path: %w", err)
	}

	return nil
}

// GetCommentSubtree は指定したコメントとその全子孫を返します。
//
// SQL は comment_subtree.sql から読み込みます。
// 指定コメントの post_id と path を先に取得し、その値を条件として渡すことで
// 「同じ投稿に属し、同じパスプレフィックスを持つ全コメント」を一発で取得します。
// 例: path = "/1/2/" → LIKE '/1/2/%' でコメント4（path="/1/2/4/"）も取得できる
func (r *Repository) GetCommentSubtree(ctx context.Context, commentID int64) ([]Comment, error) {
	// サブツリーのルートとなるコメントの post_id と path を先に取得する
	var (
		postID int64
		path   string
	)
	if err := r.db.QueryRowContext(ctx, `SELECT post_id, path FROM comments WHERE id = ?`, commentID).Scan(&postID, &path); err != nil {
		return nil, fmt.Errorf("get subtree root info: %w", err)
	}

	query, err := r.loadQuery("comment_subtree.sql")
	if err != nil {
		return nil, err
	}
	// post_id と path を条件として渡し、複合インデックスに沿って絞り込む
	return r.queryComments(ctx, query, postID, path)
}

// GetAncestorChain は指定したコメントのルートから自分自身までの祖先チェーンを返します。
//
// SQL は ancestor_chain.sql から読み込みます。
// Materialized Path 方式では path 文字列を解析して祖先 ID を取り出し、
// それらのコメントを深さ順に返します。
func (r *Repository) GetAncestorChain(ctx context.Context, commentID int64) ([]Comment, error) {
	query, err := r.loadQuery("ancestor_chain.sql")
	if err != nil {
		return nil, err
	}
	return r.queryComments(ctx, query, commentID)
}

// GetPostCommentTree は指定した投稿の全コメントをツリー構造で返します。
//
// SQL は post_comment_tree.sql から読み込みます。
// path 順（= ツリーの深さ優先順）で取得した後、
// メモリ上で commenttree.BuildTree を呼んでツリー構造に変換します。
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
