// このファイルは Materialized Path 方式（pathtree パッケージ）のリポジトリに対する
// integration test を実装します。
//
// テスト方針:
//   - 実際の MySQL に接続してテストを実行する（モックは使用しない）
//   - 各テスト関数の先頭で LockDatabase + ResetSchema を呼び、テスト間の独立性を確保する
//   - Materialized Path 特有の動作（path の自動組み立て、LIKE クエリ）を重点的に検証する
package pathtree_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"private-comment-tree-sample/internal/pathtree"
	"private-comment-tree-sample/test/testdb"
)

// TestRepositoryCreateComment は CreateComment メソッドのテストです。
//
// Materialized Path 方式では、コメント挿入時に path と depth が
// 自動的に計算・設定されることを確認します。
// path の形式: "/親のID1/親のID2/.../自分のID/"
func TestRepositoryCreateComment(t *testing.T) {
	// 並行テスト実行を防ぐためにDBロックを取得し、スキーマをリセットする
	testdb.LockDatabase(t)
	db := testdb.OpenMySQL(t)
	defer db.Close()
	testdb.ResetSchema(t, db)

	repo := pathtree.NewRepository(db, testdb.SQLQuerySubDir(t, "path"))

	t.Run("コメント作成_pathとdepthを更新する", func(t *testing.T) {
		// 初期データのツリー（投稿1）:
		//   id=1（path="/1/"）→ id=2（path="/1/2/"）→ id=4（path="/1/2/4/"）
		//   id=1（path="/1/"）→ id=3（path="/1/3/"）
		//
		// id=2 の子として新コメントを追加すると:
		//   depth = parent_depth + 1 = 1 + 1 = 2
		//   path = parent_path + new_id + "/" = "/1/2/" + "{new_id}" + "/"
		parentID := int64(2)
		comment := &pathtree.Comment{
			PostID:    1,
			ParentID:  &parentID,
			Body:      "comment5",
			CreatedAt: time.Date(2026, 1, 1, 10, 5, 0, 0, time.UTC),
		}

		err := repo.CreateComment(context.Background(), comment)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if comment.ID == 0 {
			t.Fatal("expected inserted comment id to be set")
		}

		// path が正しく更新されているか確認するため、サブツリー取得で5件になることを確認する
		subtree, err := repo.GetCommentSubtree(context.Background(), 1)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(subtree) != 5 {
			t.Fatalf("expected 5 comments, got %d", len(subtree))
		}

		// 新コメントの祖先チェーンを確認する（path 解析による: 1→2→新コメント）
		ancestors, err := repo.GetAncestorChain(context.Background(), comment.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		expected := []int64{1, 2, comment.ID}
		if len(ancestors) != len(expected) {
			t.Fatalf("expected %d ancestors, got %d", len(expected), len(ancestors))
		}
		for i, want := range expected {
			if ancestors[i].ID != want {
				t.Fatalf("expected ancestor id %d at index %d, got %d", want, i, ancestors[i].ID)
			}
		}
	})

	t.Run("コメント作成_親コメント不正", func(t *testing.T) {
		// 存在しない親コメント ID を指定した場合は ErrParentNotFound を返すことを確認
		// pathtree では ErrNoRows を ErrParentNotFound に変換する
		parentID := int64(999)
		comment := &pathtree.Comment{
			PostID:    1,
			ParentID:  &parentID,
			Body:      "invalid child",
			CreatedAt: time.Date(2026, 1, 1, 10, 6, 0, 0, time.UTC),
		}

		err := repo.CreateComment(context.Background(), comment)
		if !errors.Is(err, pathtree.ErrParentNotFound) {
			t.Fatalf("expected ErrParentNotFound, got %v", err)
		}
	})
}

// TestRepositoryGetCommentSubtree は GetCommentSubtree メソッドのテストです。
//
// Materialized Path 方式では、対象コメントの path を取得してから
// LIKE 'path%' クエリでサブツリーを一発取得します。
//
// 初期データのツリー構造（投稿1）:
//
//	id=1（path="/1/"）
//	├── id=2（path="/1/2/"）
//	│   └── id=4（path="/1/2/4/"）
//	└── id=3（path="/1/3/"）
func TestRepositoryGetCommentSubtree(t *testing.T) {
	testdb.LockDatabase(t)
	db := testdb.OpenMySQL(t)
	defer db.Close()
	testdb.ResetSchema(t, db)

	repo := pathtree.NewRepository(db, testdb.SQLQuerySubDir(t, "path"))

	t.Run("サブツリー取得_pathのprefixで取得", func(t *testing.T) {
		// コメント1（path="/1/"）のサブツリー:
		// LIKE '/1/%' にマッチするコメント = id=1,2,3,4 の4件
		// （コメント1自身の path="/1/" も LIKE '/1/%' にマッチすることに注意）
		comments, err := repo.GetCommentSubtree(context.Background(), 1)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		expected := []int64{1, 2, 3, 4}
		if len(comments) != len(expected) {
			t.Fatalf("expected %d comments, got %d", len(expected), len(comments))
		}
		for i, want := range expected {
			if comments[i].ID != want {
				t.Fatalf("expected comment id %d at index %d, got %d", want, i, comments[i].ID)
			}
		}
	})
}

// TestCommentSubtreeSQL は Materialized Path 方式のサブツリー取得SQLの構造を検証します。
//
// 既存スキーマでは idx_comments_post_path (post_id, path(255)) を定義しているため、
// SQL 側も post_id を条件に含めて左端列から使える形に揃えておく必要があります。
func TestCommentSubtreeSQL(t *testing.T) {
	t.Run("サブツリーSQL_post_idとpath_prefixで絞り込む", func(t *testing.T) {
		path := filepath.Join(testdb.SQLQuerySubDir(t, "path"), "comment_subtree.sql")
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read sql file: %v", err)
		}

		sqlText := strings.ToLower(string(content))
		if !strings.Contains(sqlText, "where post_id = ?") {
			t.Fatal("expected subtree sql to filter by post_id")
		}
		if !strings.Contains(sqlText, "path like concat(?, '%')") {
			t.Fatal("expected subtree sql to filter by path prefix")
		}
	})
}

// TestRepositoryGetAncestorChain は GetAncestorChain メソッドのテストです。
//
// Materialized Path 方式では、指定コメントの path 文字列（例: "/1/2/4/"）を
// 分解して各 ID のコメントを取得することで祖先チェーンを返します。
func TestRepositoryGetAncestorChain(t *testing.T) {
	testdb.LockDatabase(t)
	db := testdb.OpenMySQL(t)
	defer db.Close()
	testdb.ResetSchema(t, db)

	repo := pathtree.NewRepository(db, testdb.SQLQuerySubDir(t, "path"))

	t.Run("祖先チェーン取得_path前方一致で返す", func(t *testing.T) {
		// コメント4（path="/1/2/4/"）の祖先チェーン:
		// path 内の ID = [1, 2, 4] をルート順に返す
		comments, err := repo.GetAncestorChain(context.Background(), 4)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		expected := []int64{1, 2, 4}
		if len(comments) != len(expected) {
			t.Fatalf("expected %d comments, got %d", len(expected), len(comments))
		}
		for i, want := range expected {
			if comments[i].ID != want {
				t.Fatalf("expected ancestor id %d at index %d, got %d", want, i, comments[i].ID)
			}
		}
	})
}

// TestRepositoryGetPostCommentTree は GetPostCommentTree メソッドのテストです。
//
// Materialized Path 方式でも、ツリー構造への変換は commenttree.BuildTree に委譲します。
// path 順（= 辞書順）で取得することでツリーの深さ優先順序が得られます。
func TestRepositoryGetPostCommentTree(t *testing.T) {
	testdb.LockDatabase(t)
	db := testdb.OpenMySQL(t)
	defer db.Close()
	testdb.ResetSchema(t, db)

	repo := pathtree.NewRepository(db, testdb.SQLQuerySubDir(t, "path"))

	t.Run("投稿ツリー取得_path順で木構造へ変換", func(t *testing.T) {
		nodes, err := repo.GetPostCommentTree(context.Background(), 1)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		// 投稿1のルートノードは id=1 の1件
		if len(nodes) != 1 {
			t.Fatalf("expected 1 root node, got %d", len(nodes))
		}
		if nodes[0].Comment.ID != 1 {
			t.Fatalf("expected root id 1, got %d", nodes[0].Comment.ID)
		}
		// ルートの直接の子は id=2 と id=3 の2件
		if len(nodes[0].Children) != 2 {
			t.Fatalf("expected 2 child nodes, got %d", len(nodes[0].Children))
		}
	})
}
