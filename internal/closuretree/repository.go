// Package closuretree は、閉包テーブル（Closure Table）方式でコメントツリーを管理するパッケージです。
//
// 閉包テーブル方式の特徴:
//   - comment_closures テーブルに「全ての祖先・子孫ペア」を事前に展開して保持する
//   - サブツリー・祖先チェーンの取得が JOIN 一発で完了し、再帰クエリが不要
//   - 書き込み時に複数の closure 行を INSERT するため、トランザクションが必須
//   - ノード数 N に対して closure 行数は O(N²) になりうるため、深いツリーでは注意が必要
//
// comment_closures テーブルの構造:
//
//	(ancestor_id, descendant_id, depth)
//	- 自己参照行（depth=0）を含む全祖先・子孫ペアを保持する
//	- depth はその祖先から子孫までの距離
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

// ErrParentNotFound は指定した親コメントIDが存在しない場合に返されるエラーです。
var ErrParentNotFound = errors.New("parent comment not found")

// commenttree パッケージの型をこのパッケージのエイリアスとして公開します。
// 呼び出し元が closuretree.Comment として使えるようにするための型エイリアスです。
type Comment = commenttree.Comment
type CommentNode = commenttree.CommentNode

// Repository は閉包テーブル方式のコメントリポジトリです。
//
// SQL クエリをファイルから読み込む設計にすることで、
// Go コードと SQL を分離して管理しやすくしています。
type Repository struct {
	// db はデータベース接続。複数のゴルーチンから安全に共有できます。
	db *sql.DB
	// sqlRoot は SQL クエリファイルが格納されたディレクトリのパス。
	// 実行時は "sql/queries/closure"、テスト時は testdb.SQLQuerySubDir(t, "closure") で解決されます。
	sqlRoot string
}

// NewRepository は新しい Repository を生成して返します。
func NewRepository(db *sql.DB, sqlRoot string) *Repository {
	return &Repository{db: db, sqlRoot: sqlRoot}
}

// CreateComment は新しいコメントをデータベースに挿入し、閉包テーブルを更新します。
//
// 閉包テーブル方式では以下の操作をトランザクション内で実行します:
//  1. comments テーブルにコメントを INSERT する
//  2. 採番された ID を使って path を確定し UPDATE する
//  3. comment_closures に自己参照行（ancestor=self, descendant=self, depth=0）を INSERT する
//  4. 親コメントが存在する場合、親の全祖先に対して新コメントへの closure 行を INSERT する
//     （"親の全祖先" は comment_closures から WHERE descendant_id = parent_id で取得できる）
//
// 挿入成功後、comment.ID / comment.Depth / comment.Path が設定されます。
func (r *Repository) CreateComment(ctx context.Context, comment *Comment) error {
	if comment == nil {
		return errors.New("comment is nil")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	// エラー時は自動的にロールバック（Commit が呼ばれていれば no-op になる）
	defer tx.Rollback()

	// 親コメントの情報取得と存在チェック
	// ルートコメントの場合はデフォルト値（path="/"、depth=0）を使う
	pathPrefix := "/"
	depth := 0
	if comment.ParentID != nil {
		var (
			count       int
			parentPath  string
			parentDepth int
		)
		// COUNT(*) で存在確認と、COALESCE で path/depth の NULL を安全に処理する
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

	// コメントを INSERT する（path はまだ仮の値で後から UPDATE する）
	result, err := tx.ExecContext(
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
	comment.Path = fmt.Sprintf("%s%d/", pathPrefix, comment.ID)

	// 確定した path で UPDATE する（INSERT 時は ID が不明なため 2 ステップになる）
	if _, err := tx.ExecContext(
		ctx,
		`UPDATE comments SET path = ? WHERE id = ?`,
		comment.Path,
		comment.ID,
	); err != nil {
		return fmt.Errorf("update comment path: %w", err)
	}

	// 自己参照の closure 行を追加する（depth=0）
	// 全てのコメントは自分自身の ancestor かつ descendant でもある
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO comment_closures (ancestor_id, descendant_id, depth) VALUES (?, ?, 0)`,
		comment.ID,
		comment.ID,
	); err != nil {
		return fmt.Errorf("insert self closure: %w", err)
	}

	// 親コメントの全祖先に対して新コメントへの closure 行を追加する。
	//
	// サブクエリで "親の全祖先" を取得し、それぞれに対して
	// （ancestor_id=祖先, descendant_id=新コメント, depth=祖先からの距離+1）を INSERT する。
	// これにより、ルートコメントから新コメントまでの全パスが closure テーブルに記録される。
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

// GetCommentSubtree は指定したコメントとその全子孫を返します。
//
// SQL は comment_subtree.sql から読み込みます。
// comment_closures テーブルを JOIN することで再帰クエリなしにサブツリーを取得します。
func (r *Repository) GetCommentSubtree(ctx context.Context, commentID int64) ([]Comment, error) {
	query, err := r.loadQuery("comment_subtree.sql")
	if err != nil {
		return nil, err
	}
	return r.queryComments(ctx, query, commentID)
}

// GetAncestorChain は指定したコメントのルートから自分自身までの祖先チェーンを返します。
//
// SQL は ancestor_chain.sql から読み込みます。
// 戻り値はルートコメント（depth=0 に最も近い）から順に並びます。
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
// データベースから全コメントをフラットに取得した後、
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
