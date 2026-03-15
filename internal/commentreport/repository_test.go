package commentreport_test

import (
	"context"
	"testing"
	"time"

	"private-comment-tree-sample/internal/commentreport"
	"private-comment-tree-sample/test/testdb"
)

func TestRepositoryGetPostDiscussionSnapshot(t *testing.T) {
	testdb.LockDatabase(t)
	db := testdb.OpenMySQL(t)
	defer db.Close()
	testdb.ResetSchema(t, db)

	repo := commentreport.NewRepository(db, testdb.SQLQuerySubDir(t, "business"))

	t.Run("投稿サマリ取得_コメント件数と最終活動日時を返す", func(t *testing.T) {
		snapshot, err := repo.GetPostDiscussionSnapshot(context.Background(), 1)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if snapshot.PostID != 1 {
			t.Fatalf("expected post id 1, got %d", snapshot.PostID)
		}
		if snapshot.PostTitle != "PostA" {
			t.Fatalf("expected title PostA, got %s", snapshot.PostTitle)
		}
		if snapshot.RootCommentCount != 1 {
			t.Fatalf("expected 1 root comment, got %d", snapshot.RootCommentCount)
		}
		if snapshot.TotalCommentCount != 4 {
			t.Fatalf("expected 4 total comments, got %d", snapshot.TotalCommentCount)
		}
		if snapshot.MaxDepth != 2 {
			t.Fatalf("expected max depth 2, got %d", snapshot.MaxDepth)
		}
		wantLatest := time.Date(2026, 1, 1, 10, 4, 0, 0, time.UTC)
		if !snapshot.LatestCommentAt.Equal(wantLatest) {
			t.Fatalf("expected latest activity %v, got %v", wantLatest, snapshot.LatestCommentAt)
		}
	})
}

func TestRepositoryListRootThreadSummaries(t *testing.T) {
	testdb.LockDatabase(t)
	db := testdb.OpenMySQL(t)
	defer db.Close()
	testdb.ResetSchema(t, db)

	repo := commentreport.NewRepository(db, testdb.SQLQuerySubDir(t, "business"))

	t.Run("ルートスレッドサマリ取得_子孫数と活動日時を返す", func(t *testing.T) {
		summaries, err := repo.ListRootThreadSummaries(context.Background(), 1)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(summaries) != 1 {
			t.Fatalf("expected 1 summary, got %d", len(summaries))
		}

		summary := summaries[0]
		if summary.RootCommentID != 1 {
			t.Fatalf("expected root comment id 1, got %d", summary.RootCommentID)
		}
		if summary.RootBody != "comment1" {
			t.Fatalf("expected root body comment1, got %s", summary.RootBody)
		}
		if summary.DirectReplyCount != 2 {
			t.Fatalf("expected 2 direct replies, got %d", summary.DirectReplyCount)
		}
		if summary.DescendantCount != 3 {
			t.Fatalf("expected 3 descendants, got %d", summary.DescendantCount)
		}
		if summary.MaxDepth != 2 {
			t.Fatalf("expected max depth 2, got %d", summary.MaxDepth)
		}
		wantLatest := time.Date(2026, 1, 1, 10, 4, 0, 0, time.UTC)
		if !summary.LatestReplyAt.Equal(wantLatest) {
			t.Fatalf("expected latest reply at %v, got %v", wantLatest, summary.LatestReplyAt)
		}
	})
}
