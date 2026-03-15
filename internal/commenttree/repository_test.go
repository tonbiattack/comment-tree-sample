package commenttree_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"private-comment-tree-sample/internal/commenttree"
	"private-comment-tree-sample/test/testdb"
)

func TestRepositoryCreateComment(t *testing.T) {
	testdb.LockDatabase(t)
	db := testdb.OpenMySQL(t)
	defer db.Close()
	testdb.ResetSchema(t, db)

	repo := commenttree.NewRepository(db, testdb.SQLQueryDir(t))

	t.Run("コメント作成_ルートコメント", func(t *testing.T) {
		comment := &commenttree.Comment{
			PostID:    1,
			Body:      "new root comment",
			CreatedAt: time.Date(2026, 1, 1, 10, 5, 0, 0, time.UTC),
		}

		err := repo.CreateComment(context.Background(), comment)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if comment.ID == 0 {
			t.Fatal("expected inserted comment id to be set")
		}
	})

	t.Run("コメント作成_親コメント不正", func(t *testing.T) {
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

func TestRepositoryGetRootCommentsByPostID(t *testing.T) {
	testdb.LockDatabase(t)
	db := testdb.OpenMySQL(t)
	defer db.Close()
	testdb.ResetSchema(t, db)

	repo := commenttree.NewRepository(db, testdb.SQLQueryDir(t))

	t.Run("ルートコメント取得_投稿配下", func(t *testing.T) {
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

func TestRepositoryGetCommentSubtree(t *testing.T) {
	testdb.LockDatabase(t)
	db := testdb.OpenMySQL(t)
	defer db.Close()
	testdb.ResetSchema(t, db)

	repo := commenttree.NewRepository(db, testdb.SQLQueryDir(t))

	t.Run("サブツリー取得_子孫を深さ順に取得", func(t *testing.T) {
		comments, err := repo.GetCommentSubtree(context.Background(), 1)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(comments) != 4 {
			t.Fatalf("expected 4 comments, got %d", len(comments))
		}

		expected := []int64{1, 2, 3, 4}
		for i, want := range expected {
			if comments[i].ID != want {
				t.Fatalf("expected comment id %d at index %d, got %d", want, i, comments[i].ID)
			}
		}
	})
}

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
		if len(nodes) != 1 {
			t.Fatalf("expected 1 root node, got %d", len(nodes))
		}

		root := nodes[0]
		if root.Comment.ID != 1 {
			t.Fatalf("expected root id 1, got %d", root.Comment.ID)
		}
		if len(root.Children) != 2 {
			t.Fatalf("expected 2 child nodes, got %d", len(root.Children))
		}
		if root.Children[0].Comment.ID != 2 || root.Children[1].Comment.ID != 3 {
			t.Fatalf("expected child ids [2 3], got [%d %d]", root.Children[0].Comment.ID, root.Children[1].Comment.ID)
		}
		if len(root.Children[0].Children) != 1 || root.Children[0].Children[0].Comment.ID != 4 {
			t.Fatalf("expected grandchild id 4 under comment 2")
		}
	})
}
