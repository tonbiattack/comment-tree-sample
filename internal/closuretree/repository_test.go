package closuretree_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"private-comment-tree-sample/internal/closuretree"
	"private-comment-tree-sample/test/testdb"
)

func TestRepositoryCreateComment(t *testing.T) {
	testdb.LockDatabase(t)
	db := testdb.OpenMySQL(t)
	defer db.Close()
	testdb.ResetSchema(t, db)

	repo := closuretree.NewRepository(db, testdb.SQLQuerySubDir(t, "closure"))

	t.Run("コメント作成_閉包テーブルも更新する", func(t *testing.T) {
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

		subtree, err := repo.GetCommentSubtree(context.Background(), 1)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(subtree) != 5 {
			t.Fatalf("expected 5 comments, got %d", len(subtree))
		}

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

func TestRepositoryGetCommentSubtree(t *testing.T) {
	testdb.LockDatabase(t)
	db := testdb.OpenMySQL(t)
	defer db.Close()
	testdb.ResetSchema(t, db)

	repo := closuretree.NewRepository(db, testdb.SQLQuerySubDir(t, "closure"))

	t.Run("サブツリー取得_閉包テーブルから取得", func(t *testing.T) {
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

func TestRepositoryGetAncestorChain(t *testing.T) {
	testdb.LockDatabase(t)
	db := testdb.OpenMySQL(t)
	defer db.Close()
	testdb.ResetSchema(t, db)

	repo := closuretree.NewRepository(db, testdb.SQLQuerySubDir(t, "closure"))

	t.Run("祖先チェーン取得_ルートから自分まで返す", func(t *testing.T) {
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
		if len(nodes) != 1 {
			t.Fatalf("expected 1 root node, got %d", len(nodes))
		}
		if nodes[0].Comment.ID != 1 {
			t.Fatalf("expected root id 1, got %d", nodes[0].Comment.ID)
		}
		if len(nodes[0].Children) != 2 {
			t.Fatalf("expected 2 child nodes, got %d", len(nodes[0].Children))
		}
	})
}
