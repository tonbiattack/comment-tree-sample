// main パッケージは、3つのツリー実装方式とビジネスロジックの動作を
// 統合的にデモンストレーションするサンプルプログラムです。
//
// 実行前の準備:
//  1. Docker Compose で MySQL を起動する: docker compose up -d mysql
//  2. リポジトリルートで実行する: go run ./cmd/samplefetch
//
// 出力内容:
//   - adjacency（隣接リスト）: ルートコメント・フルツリー
//   - closure（閉包テーブル）: サブツリー・祖先チェーン・フルツリー
//   - path（Materialized Path）: サブツリー・祖先チェーン・フルツリー
//   - business（ビジネスロジック）: 投稿スナップショット・スレッドサマリ・未返信スレッド・活動順投稿
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql" // MySQL ドライバーを database/sql に登録する

	"private-comment-tree-sample/internal/closuretree"
	"private-comment-tree-sample/internal/commentreport"
	"private-comment-tree-sample/internal/commenttree"
	"private-comment-tree-sample/internal/mysqlconn"
	"private-comment-tree-sample/internal/pathtree"
)

func main() {
	// カレントディレクトリ（リポジトリルート）を基準に MySQL 接続設定を解決する
	cfg := mysqlconn.Resolve(".")
	db, err := sql.Open("mysql", cfg.DSN(false))
	if err != nil {
		log.Fatalf("open mysql: %v", err)
	}
	defer db.Close()

	// 3つのツリー実装方式とビジネスロジックのリポジトリを初期化する
	// sqlRoot には各方式の SQL クエリファイルが格納されたディレクトリを指定する
	adjacencyRepo := commenttree.NewRepository(db, "sql/queries")          // 隣接リスト方式
	closureRepo := closuretree.NewRepository(db, "sql/queries/closure")    // 閉包テーブル方式
	pathRepo := pathtree.NewRepository(db, "sql/queries/path")             // Materialized Path 方式
	reportRepo := commentreport.NewRepository(db, "sql/queries/business")  // ビジネスロジック

	// ─────────────────────────────────────────
	// 1. 隣接リスト方式のデモ
	// ─────────────────────────────────────────

	// 投稿1のルートコメント一覧を取得する（parent_id IS NULL のもの）
	roots, err := adjacencyRepo.GetRootCommentsByPostID(context.Background(), 1)
	if err != nil {
		log.Fatalf("get root comments: %v", err)
	}
	fmt.Println("adjacency root comments:")
	for _, root := range roots {
		fmt.Printf("- id=%d body=%s\n", root.ID, root.Body)
	}

	// 投稿1のコメント全体をツリー構造で取得する
	// SQL で全コメントをフラットに取得 → メモリ上で BuildTree によりツリー化する
	tree, err := adjacencyRepo.GetPostCommentTree(context.Background(), 1)
	if err != nil {
		log.Fatalf("get adjacency post comment tree: %v", err)
	}
	fmt.Println("adjacency full tree:")
	printNodes(tree, 0)

	// ─────────────────────────────────────────
	// 2. 閉包テーブル方式のデモ
	// ─────────────────────────────────────────

	// コメント1のサブツリーを取得する（comment_closures テーブルを JOIN）
	closureSubtree, err := closureRepo.GetCommentSubtree(context.Background(), 1)
	if err != nil {
		log.Fatalf("get closure subtree: %v", err)
	}
	fmt.Println("closure subtree from comment 1:")
	for _, comment := range closureSubtree {
		fmt.Printf("- id=%d body=%s\n", comment.ID, comment.Body)
	}

	// コメント4の祖先チェーンを取得する（ルートから自分まで: 1→2→4）
	ancestors, err := closureRepo.GetAncestorChain(context.Background(), 4)
	if err != nil {
		log.Fatalf("get closure ancestors: %v", err)
	}
	fmt.Println("closure ancestor chain for comment 4:")
	for _, comment := range ancestors {
		fmt.Printf("- id=%d body=%s\n", comment.ID, comment.Body)
	}

	// 投稿1のフルツリーを閉包テーブル方式で取得する
	closureTree, err := closureRepo.GetPostCommentTree(context.Background(), 1)
	if err != nil {
		log.Fatalf("get closure post comment tree: %v", err)
	}
	fmt.Println("closure full tree:")
	printNodes(closureTree, 0)

	// ─────────────────────────────────────────
	// 3. Materialized Path 方式のデモ
	// ─────────────────────────────────────────

	// コメント1のサブツリーを取得する（path の LIKE プレフィックスマッチ）
	pathSubtree, err := pathRepo.GetCommentSubtree(context.Background(), 1)
	if err != nil {
		log.Fatalf("get path subtree: %v", err)
	}
	fmt.Println("path subtree from comment 1:")
	for _, comment := range pathSubtree {
		fmt.Printf("- id=%d body=%s\n", comment.ID, comment.Body)
	}

	// コメント4の祖先チェーンを path 文字列の解析で取得する（1→2→4）
	pathAncestors, err := pathRepo.GetAncestorChain(context.Background(), 4)
	if err != nil {
		log.Fatalf("get path ancestors: %v", err)
	}
	fmt.Println("path ancestor chain for comment 4:")
	for _, comment := range pathAncestors {
		fmt.Printf("- id=%d body=%s\n", comment.ID, comment.Body)
	}

	// 投稿1のフルツリーを Materialized Path 方式で取得する
	pathTree, err := pathRepo.GetPostCommentTree(context.Background(), 1)
	if err != nil {
		log.Fatalf("get path post comment tree: %v", err)
	}
	fmt.Println("path full tree:")
	printNodes(pathTree, 0)

	// ─────────────────────────────────────────
	// 4. ビジネスロジックのデモ
	// ─────────────────────────────────────────

	// 投稿1のコメント統計スナップショットを取得する
	// （ルートコメント数・総コメント数・最大深さ・最終活動日時）
	snapshot, err := reportRepo.GetPostDiscussionSnapshot(context.Background(), 1)
	if err != nil {
		log.Fatalf("get post discussion snapshot: %v", err)
	}
	fmt.Printf("business snapshot: post_id=%d title=%s roots=%d total=%d max_depth=%d latest=%s\n",
		snapshot.PostID,
		snapshot.PostTitle,
		snapshot.RootCommentCount,
		snapshot.TotalCommentCount,
		snapshot.MaxDepth,
		snapshot.LatestCommentAt.Format(time.RFC3339),
	)

	// 投稿1の各ルートスレッドのサマリを取得する
	// （直属返信数・全子孫数・最大深さ・最終返信日時）
	threadSummaries, err := reportRepo.ListRootThreadSummaries(context.Background(), 1)
	if err != nil {
		log.Fatalf("list root thread summaries: %v", err)
	}
	fmt.Println("business root thread summaries:")
	for _, summary := range threadSummaries {
		fmt.Printf("- root_id=%d direct=%d descendants=%d max_depth=%d latest=%s\n",
			summary.RootCommentID,
			summary.DirectReplyCount,
			summary.DescendantCount,
			summary.MaxDepth,
			summary.LatestReplyAt.Format(time.RFC3339),
		)
	}

	// 投稿2の未返信ルートスレッドを取得する（direct_reply_count = 0 のもの）
	unansweredThreads, err := reportRepo.ListUnansweredRootThreads(context.Background(), 2)
	if err != nil {
		log.Fatalf("list unanswered root threads: %v", err)
	}
	fmt.Println("business unanswered root threads for post 2:")
	for _, summary := range unansweredThreads {
		fmt.Printf("- root_id=%d direct=%d descendants=%d\n",
			summary.RootCommentID,
			summary.DirectReplyCount,
			summary.DescendantCount,
		)
	}

	// 全投稿を最新コメント日時順に取得する（コメントなしの投稿はリスト末尾に来る）
	postsByActivity, err := reportRepo.ListPostsByRecentActivity(context.Background())
	if err != nil {
		log.Fatalf("list posts by recent activity: %v", err)
	}
	fmt.Println("business posts by recent activity:")
	for _, post := range postsByActivity {
		latest := "-"
		if !post.LatestCommentAt.IsZero() {
			latest = post.LatestCommentAt.Format(time.RFC3339)
		}
		fmt.Printf("- post_id=%d title=%s total=%d latest=%s\n",
			post.PostID,
			post.PostTitle,
			post.TotalCommentCount,
			latest,
		)
	}
}

// printNodes はコメントノードのツリーをインデント付きで再帰的に表示します。
//
// depth はインデントレベル（0始まり）。
// 各レベルにつき 2 スペースのインデントを追加します。
func printNodes(nodes []*commenttree.CommentNode, depth int) {
	prefix := ""
	for i := 0; i < depth; i++ {
		prefix += "  "
	}

	for _, node := range nodes {
		fmt.Printf("%s- id=%d body=%s\n", prefix, node.Comment.ID, node.Comment.Body)
		// 子ノードを再帰的に表示する（depth を +1 してインデントを深くする）
		printNodes(node.Children, depth+1)
	}
}
