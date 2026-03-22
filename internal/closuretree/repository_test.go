// このファイルは閉包テーブル方式（closuretree パッケージ）のリポジトリに対する
// integration test を実装します。
//
// テスト方針:
//   - 実際の MySQL に接続してテストを実行する（モックは使用しない）
//   - 各テスト関数の先頭で LockDatabase + ResetSchema を呼び、テスト間の独立性を確保する
//   - 閉包テーブル方式特有の動作（closure 行の自動管理）を重点的に検証する
package closuretree_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"private-comment-tree-sample/internal/closuretree"
	"private-comment-tree-sample/test/testdb"
)

// TestRepositoryCreateComment は CreateComment メソッドのテストです。
//
// 閉包テーブル方式では、コメント挿入時に comment_closures テーブルも
// 自動的に更新される必要があります。
// 具体的には以下の closure 行が追加されます:
//   - 自己参照行: (new_id, new_id, 0)
//   - 親の全祖先への行: 親の closures から depth+1 で継承
func TestRepositoryCreateComment(t *testing.T) {
	// 並行テスト実行を防ぐためにDBロックを取得し、スキーマをリセットする
	testdb.LockDatabase(t)
	db := testdb.OpenMySQL(t)
	defer db.Close()
	testdb.ResetSchema(t, db)

	repo := closuretree.NewRepository(db, testdb.SQLQuerySubDir(t, "closure"))

	t.Run("コメント作成_閉包テーブルも更新する", func(t *testing.T) {
		// 初期データのツリー（投稿1）:
		//   id=1 → id=2 → id=4
		//       └── id=3
		//
		// id=2 の子として id=5（新コメント）を追加する。
		// 期待される closure 行:
		//   (5, 5, 0) ← 自己参照
		//   (2, 5, 1) ← 親への参照（depth=1）
		//   (1, 5, 2) ← 祖父への参照（depth=2）
		parentID := int64(2)
		comment := &closuretree.Comment{
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

		// closure テーブルが正しく更新されているか確認するため、
		// コメント1のサブツリーが5件（元の4件 + 新規1件）になっていることを確認する
		subtree, err := repo.GetCommentSubtree(context.Background(), 1)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(subtree) != 5 {
			t.Fatalf("expected 5 comments, got %d", len(subtree))
		}

		// 新コメントの祖先チェーンを確認する（ルートから自分まで: 1→2→新コメント）
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
		parentID := int64(999)
		comment := &closuretree.Comment{
			PostID:    1,
			ParentID:  &parentID,
			Body:      "invalid child",
			CreatedAt: time.Date(2026, 1, 1, 10, 6, 0, 0, time.UTC),
		}

		err := repo.CreateComment(context.Background(), comment)
		if !errors.Is(err, closuretree.ErrParentNotFound) {
			t.Fatalf("expected ErrParentNotFound, got %v", err)
		}
	})
}

// TestRepositoryGetCommentSubtree は GetCommentSubtree メソッドのテストです。
//
// 閉包テーブル方式では、comment_closures テーブルを JOIN して
// サブツリーを再帰クエリなしに取得します。
//
// 初期データのツリー構造（投稿1）:
//
//	id=1（ルート）
//	├── id=2（親=1）
//	│   └── id=4（親=2）
//	└── id=3（親=1）
func TestRepositoryGetCommentSubtree(t *testing.T) {
	testdb.LockDatabase(t)
	db := testdb.OpenMySQL(t)
	defer db.Close()
	testdb.ResetSchema(t, db)

	repo := closuretree.NewRepository(db, testdb.SQLQuerySubDir(t, "closure"))

	t.Run("サブツリー取得_閉包テーブルから取得", func(t *testing.T) {
		// コメント1のサブツリー = id=1,2,3,4 の4件が depth 昇順で返される
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

// TestRepositoryGetAncestorChain は GetAncestorChain メソッドのテストです。
//
// 閉包テーブル方式では、指定コメントの descendant_id に一致する closure 行を
// 全て取得することで、再帰なしに祖先チェーンを取得できます。
func TestRepositoryGetAncestorChain(t *testing.T) {
	testdb.LockDatabase(t)
	db := testdb.OpenMySQL(t)
	defer db.Close()
	testdb.ResetSchema(t, db)

	repo := closuretree.NewRepository(db, testdb.SQLQuerySubDir(t, "closure"))

	t.Run("祖先チェーン取得_ルートから自分まで返す", func(t *testing.T) {
		// コメント4の祖先チェーン（depth 昇順）: id=1（ルート）→ id=2 → id=4（自分）
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
// 閉包テーブル方式でも、ツリー構造への変換は commenttree.BuildTree に委譲します。
// このテストでは正しくツリーが組み立てられているかを確認します。
func TestRepositoryGetPostCommentTree(t *testing.T) {
	testdb.LockDatabase(t)
	db := testdb.OpenMySQL(t)
	defer db.Close()
	testdb.ResetSchema(t, db)

	repo := closuretree.NewRepository(db, testdb.SQLQuerySubDir(t, "closure"))

	t.Run("投稿ツリー取得_木構造へ変換", func(t *testing.T) {
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
