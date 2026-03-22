// このファイルはビジネスロジックリポジトリ（commentreport パッケージ）に対する
// integration test を実装します。
//
// テスト方針:
//   - 実際の MySQL に接続してテストを実行する（モックは使用しない）
//   - 各テスト関数の先頭で LockDatabase + ResetSchema を呼び、テスト間の独立性を確保する
//   - 初期データ（all.sql）を前提として集計クエリの結果を検証する
//
// 初期データの概要:
//   - PostA（id=1）: コメント4件（id=1,2,3,4 でツリー構造）
//   - PostB（id=2）: コメント1件（id=5、返信なし）
//   - PostC（id=3）: コメントなし
package commentreport_test

import (
	"context"
	"testing"
	"time"

	"private-comment-tree-sample/internal/commentreport"
	"private-comment-tree-sample/test/testdb"
)

// TestRepositoryGetPostDiscussionSnapshot は GetPostDiscussionSnapshot メソッドのテストです。
//
// 投稿1（PostA）の集計結果:
//   - ルートコメント数: 1件（id=1）
//   - 総コメント数: 4件（id=1,2,3,4）
//   - 最大深さ: 2（id=4 が depth=2）
//   - 最終活動日時: 2026-01-01 10:04:00 UTC（id=4 の created_at）
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
		// parent_id IS NULL のコメントが1件（id=1）
		if snapshot.RootCommentCount != 1 {
			t.Fatalf("expected 1 root comment, got %d", snapshot.RootCommentCount)
		}
		// 投稿1の全コメントは id=1,2,3,4 の4件
		if snapshot.TotalCommentCount != 4 {
			t.Fatalf("expected 4 total comments, got %d", snapshot.TotalCommentCount)
		}
		// id=4 が最も深い（depth=2）
		if snapshot.MaxDepth != 2 {
			t.Fatalf("expected max depth 2, got %d", snapshot.MaxDepth)
		}
		// id=4 の created_at が最も新しい
		wantLatest := time.Date(2026, 1, 1, 10, 4, 0, 0, time.UTC)
		if !snapshot.LatestCommentAt.Equal(wantLatest) {
			t.Fatalf("expected latest activity %v, got %v", wantLatest, snapshot.LatestCommentAt)
		}
	})
}

// TestRepositoryListRootThreadSummaries は ListRootThreadSummaries メソッドのテストです。
//
// 投稿1のルートスレッド（id=1）の集計結果:
//   - 直属返信数（depth=1 の子）: 2件（id=2、id=3）
//   - 全子孫数: 3件（id=2、id=3、id=4）
//   - 最大深さ: 2
//   - 最終返信日時: 2026-01-01 10:04:00 UTC
func TestRepositoryListRootThreadSummaries(t *testing.T) {
	testdb.LockDatabase(t)
	db := testdb.OpenMySQL(t)
	defer db.Close()
	testdb.ResetSchema(t, db)

	repo := commentreport.NewRepository(db, testdb.SQLQuerySubDir(t, "business"))

	t.Run("ルートスレッドサマリ取得_子孫数と活動日時を返す", func(t *testing.T) {
		// 投稿1にはルートコメントが id=1 の1件のみ
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
		// id=1 への直接返信は id=2 と id=3 の2件
		if summary.DirectReplyCount != 2 {
			t.Fatalf("expected 2 direct replies, got %d", summary.DirectReplyCount)
		}
		// id=1 の子孫は id=2,3,4 の3件
		if summary.DescendantCount != 3 {
			t.Fatalf("expected 3 descendants, got %d", summary.DescendantCount)
		}
		if summary.MaxDepth != 2 {
			t.Fatalf("expected max depth 2, got %d", summary.MaxDepth)
		}
		// id=4 の created_at が最も新しい返信
		wantLatest := time.Date(2026, 1, 1, 10, 4, 0, 0, time.UTC)
		if !summary.LatestReplyAt.Equal(wantLatest) {
			t.Fatalf("expected latest reply at %v, got %v", wantLatest, summary.LatestReplyAt)
		}
	})
}

// TestRepositoryListUnansweredRootThreads は ListUnansweredRootThreads メソッドのテストです。
//
// 投稿2（PostB）のルートコメント（id=5）は直属返信が0件のため、
// 未返信スレッドとして取得されます。
func TestRepositoryListUnansweredRootThreads(t *testing.T) {
	testdb.LockDatabase(t)
	db := testdb.OpenMySQL(t)
	defer db.Close()
	testdb.ResetSchema(t, db)

	repo := commentreport.NewRepository(db, testdb.SQLQuerySubDir(t, "business"))

	t.Run("未返信ルートスレッド取得_直属返信がないルートだけ返す", func(t *testing.T) {
		// 投稿2のルートコメントは id=5 の1件で、直属返信は0件
		summaries, err := repo.ListUnansweredRootThreads(context.Background(), 2)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(summaries) != 1 {
			t.Fatalf("expected 1 summary, got %d", len(summaries))
		}

		summary := summaries[0]
		if summary.RootCommentID != 5 {
			t.Fatalf("expected root comment id 5, got %d", summary.RootCommentID)
		}
		if summary.RootBody != "comment5" {
			t.Fatalf("expected root body comment5, got %s", summary.RootBody)
		}
		// 直属返信も子孫も0件であることを確認
		if summary.DirectReplyCount != 0 {
			t.Fatalf("expected 0 direct replies, got %d", summary.DirectReplyCount)
		}
		if summary.DescendantCount != 0 {
			t.Fatalf("expected 0 descendants, got %d", summary.DescendantCount)
		}
	})
}

// TestRepositoryListPostsByRecentActivity は ListPostsByRecentActivity メソッドのテストです。
//
// 初期データの最新コメント日時:
//   - PostB（id=2）: 2026-01-01 11:00:00（id=5 の created_at）
//   - PostA（id=1）: 2026-01-01 10:04:00（id=4 の created_at）
//   - PostC（id=3）: コメントなし（LatestCommentAt = ゼロ値）
//
// 返却順序は最新コメント日時の降順。コメントなし投稿はリスト末尾に来ます。
func TestRepositoryListPostsByRecentActivity(t *testing.T) {
	testdb.LockDatabase(t)
	db := testdb.OpenMySQL(t)
	defer db.Close()
	testdb.ResetSchema(t, db)

	repo := commentreport.NewRepository(db, testdb.SQLQuerySubDir(t, "business"))

	t.Run("投稿一覧取得_最新活動順で並べる", func(t *testing.T) {
		// 全投稿3件が返される（コメントなしの PostC も含む）
		posts, err := repo.ListPostsByRecentActivity(context.Background())
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(posts) != 3 {
			t.Fatalf("expected 3 posts, got %d", len(posts))
		}
		// 最新コメント日時の降順: PostB(id=2) → PostA(id=1) → PostC(id=3)
		if posts[0].PostID != 2 || posts[1].PostID != 1 || posts[2].PostID != 3 {
			t.Fatalf("expected post ids [2 1 3], got [%d %d %d]", posts[0].PostID, posts[1].PostID, posts[2].PostID)
		}
		// PostB（id=2）のコメント数は1件
		if posts[0].TotalCommentCount != 1 {
			t.Fatalf("expected post 2 total comments 1, got %d", posts[0].TotalCommentCount)
		}
		// PostB の最新コメント日時: 2026-01-01 11:00:00 UTC
		if !posts[0].LatestCommentAt.Equal(time.Date(2026, 1, 1, 11, 0, 0, 0, time.UTC)) {
			t.Fatalf("unexpected latest comment at for post 2: %v", posts[0].LatestCommentAt)
		}
		// PostC（id=3）はコメントなしのため LatestCommentAt はゼロ値
		if !posts[2].LatestCommentAt.IsZero() {
			t.Fatalf("expected zero latest comment for post 3, got %v", posts[2].LatestCommentAt)
		}
	})
}
