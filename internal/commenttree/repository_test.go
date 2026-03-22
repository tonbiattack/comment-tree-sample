// このファイルは隣接リスト方式（commenttree パッケージ）のリポジトリに対する
// integration test を実装します。
//
// テスト方針:
//   - 実際の MySQL に接続してテストを実行する（モックは使用しない）
//   - 各テスト関数の先頭で LockDatabase + ResetSchema を呼び、テスト間の独立性を確保する
//   - 複数のテスト関数が並行実行されても LockDatabase によりシリアライズされる
package commenttree_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"private-comment-tree-sample/internal/commenttree"
	"private-comment-tree-sample/test/testdb"
)

// TestRepositoryCreateComment は CreateComment メソッドのテストです。
//
// 隣接リスト方式では path・depth を更新しないため、
// 挿入後のカラム値がDEFAULT値（path="/"、depth=0）のままであることを確認します。
func TestRepositoryCreateComment(t *testing.T) {
	// 並行テスト実行を防ぐためにDBロックを取得し、スキーマをリセットする
	testdb.LockDatabase(t)
	db := testdb.OpenMySQL(t)
	defer db.Close()
	testdb.ResetSchema(t, db)

	repo := commenttree.NewRepository(db, testdb.SQLQueryDir(t))

	t.Run("コメント作成_ルートコメント", func(t *testing.T) {
		// ルートコメントは ParentID を設定しない（NULL として挿入される）
		comment := &commenttree.Comment{
			PostID:    1,
			Body:      "new root comment",
			CreatedAt: time.Date(2026, 1, 1, 10, 5, 0, 0, time.UTC),
		}

		err := repo.CreateComment(context.Background(), comment)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		// 挿入後に ID が設定されていることを確認する
		if comment.ID == 0 {
			t.Fatal("expected inserted comment id to be set")
		}

		// 隣接リスト方式では path・depth を更新しないため DEFAULT 値のままであることを確認
		path, depth := fetchCommentPathDepth(t, db, comment.ID)
		if path != "/" {
			t.Fatalf("expected default path /, got %s", path)
		}
		if depth != 0 {
			t.Fatalf("expected default depth 0, got %d", depth)
		}
	})

	t.Run("コメント作成_子コメントでもmaterialized_pathを更新しない", func(t *testing.T) {
		// 隣接リスト方式では、子コメントでも path・depth は更新しない
		// （path/depth の管理は Materialized Path 方式・閉包テーブル方式の責務）
		parentID := int64(1)
		comment := &commenttree.Comment{
			PostID:    1,
			ParentID:  &parentID,
			Body:      "new child comment",
			CreatedAt: time.Date(2026, 1, 1, 10, 5, 30, 0, time.UTC),
		}

		err := repo.CreateComment(context.Background(), comment)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// 子コメントでも path・depth は DEFAULT 値のまま
		path, depth := fetchCommentPathDepth(t, db, comment.ID)
		if path != "/" {
			t.Fatalf("expected default path /, got %s", path)
		}
		if depth != 0 {
			t.Fatalf("expected default depth 0, got %d", depth)
		}
	})

	t.Run("コメント作成_親コメント不正", func(t *testing.T) {
		// 存在しない親コメント ID を指定した場合は ErrParentNotFound を返すことを確認
		parentID := int64(999)
		comment := &commenttree.Comment{
			PostID:    1,
			ParentID:  &parentID,
			Body:      "invalid child",
			CreatedAt: time.Date(2026, 1, 1, 10, 6, 0, 0, time.UTC),
		}

		err := repo.CreateComment(context.Background(), comment)
		if !errors.Is(err, commenttree.ErrParentNotFound) {
			t.Fatalf("expected ErrParentNotFound, got %v", err)
		}
	})
}

// fetchCommentPathDepth は指定したコメントIDの path と depth をデータベースから取得するヘルパーです。
// テスト内でデータベースの実際の状態を確認するために使用します。
func fetchCommentPathDepth(t *testing.T, db *sql.DB, commentID int64) (string, int) {
	t.Helper()

	var (
		path  string
		depth int
	)
	if err := db.QueryRow("SELECT path, depth FROM comments WHERE id = ?", commentID).Scan(&path, &depth); err != nil {
		t.Fatalf("expected comment row %d, got %v", commentID, err)
	}
	return path, depth
}

// TestRepositoryGetRootCommentsByPostID は GetRootCommentsByPostID メソッドのテストです。
//
// 初期データ（all.sql）では投稿1にルートコメント（id=1）が1件存在します。
func TestRepositoryGetRootCommentsByPostID(t *testing.T) {
	testdb.LockDatabase(t)
	db := testdb.OpenMySQL(t)
	defer db.Close()
	testdb.ResetSchema(t, db)

	repo := commenttree.NewRepository(db, testdb.SQLQueryDir(t))

	t.Run("ルートコメント取得_投稿配下", func(t *testing.T) {
		// 初期データ: 投稿1のルートコメントは id=1 の1件のみ
		comments, err := repo.GetRootCommentsByPostID(context.Background(), 1)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(comments) != 1 {
			t.Fatalf("expected 1 root comment, got %d", len(comments))
		}
		if comments[0].ID != 1 {
			t.Fatalf("expected root comment id 1, got %d", comments[0].ID)
		}
	})
}

// TestRepositoryGetCommentSubtree は GetCommentSubtree メソッドのテストです。
//
// 初期データのツリー構造（投稿1）:
//
//	id=1（ルート）
//	├── id=2（親=1）
//	│   └── id=4（親=2）
//	└── id=3（親=1）
//
// WITH RECURSIVE を使った再帰 CTE でサブツリーを取得し、
// 深さ優先順（1→2→3→4）で返されることを確認します。
func TestRepositoryGetCommentSubtree(t *testing.T) {
	testdb.LockDatabase(t)
	db := testdb.OpenMySQL(t)
	defer db.Close()
	testdb.ResetSchema(t, db)

	repo := commenttree.NewRepository(db, testdb.SQLQueryDir(t))

	t.Run("サブツリー取得_子孫を深さ順に取得", func(t *testing.T) {
		// コメント1のサブツリー = コメント1,2,3,4 の4件
		// 返却順序: 深さ優先（BFS ではなく DFS に近い順序）
		comments, err := repo.GetCommentSubtree(context.Background(), 1)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(comments) != 4 {
			t.Fatalf("expected 4 comments, got %d", len(comments))
		}

		// SQL のORDER BY により ID 順で返されることを確認
		expected := []int64{1, 2, 3, 4}
		for i, want := range expected {
			if comments[i].ID != want {
				t.Fatalf("expected comment id %d at index %d, got %d", want, i, comments[i].ID)
			}
		}
	})
}

// TestRepositoryGetPostCommentTree は GetPostCommentTree メソッドのテストです。
//
// 初期データのツリー構造（投稿1）:
//
//	id=1（ルート）
//	├── id=2（親=1）
//	│   └── id=4（親=2）
//	└── id=3（親=1）
//
// BuildTree によりメモリ上でツリー構造に変換され、
// 正しい親子関係になっていることを確認します。
func TestRepositoryGetPostCommentTree(t *testing.T) {
	testdb.LockDatabase(t)
	db := testdb.OpenMySQL(t)
	defer db.Close()
	testdb.ResetSchema(t, db)

	repo := commenttree.NewRepository(db, testdb.SQLQueryDir(t))

	t.Run("投稿ツリー取得_親子関係を木構造に変換", func(t *testing.T) {
		nodes, err := repo.GetPostCommentTree(context.Background(), 1)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		// 投稿1のルートノードは id=1 の1件
		if len(nodes) != 1 {
			t.Fatalf("expected 1 root node, got %d", len(nodes))
		}

		root := nodes[0]
		if root.Comment.ID != 1 {
			t.Fatalf("expected root id 1, got %d", root.Comment.ID)
		}
		// ルートの直接の子は id=2 と id=3 の2件
		if len(root.Children) != 2 {
			t.Fatalf("expected 2 child nodes, got %d", len(root.Children))
		}
		if root.Children[0].Comment.ID != 2 || root.Children[1].Comment.ID != 3 {
			t.Fatalf("expected child ids [2 3], got [%d %d]", root.Children[0].Comment.ID, root.Children[1].Comment.ID)
		}
		// id=2 の子は id=4 の1件（孫コメント）
		if len(root.Children[0].Children) != 1 || root.Children[0].Children[0].Comment.ID != 4 {
			t.Fatalf("expected grandchild id 4 under comment 2")
		}
	})
}
