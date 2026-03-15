package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"private-comment-tree-sample/internal/closuretree"
	"private-comment-tree-sample/internal/commentreport"
	"private-comment-tree-sample/internal/commenttree"
	"private-comment-tree-sample/internal/mysqlconn"
	"private-comment-tree-sample/internal/pathtree"
)

func main() {
	cfg := mysqlconn.Resolve(".")
	db, err := sql.Open("mysql", cfg.DSN(false))
	if err != nil {
		log.Fatalf("open mysql: %v", err)
	}
	defer db.Close()

	adjacencyRepo := commenttree.NewRepository(db, "sql/queries")
	closureRepo := closuretree.NewRepository(db, "sql/queries/closure")
	pathRepo := pathtree.NewRepository(db, "sql/queries/path")
	reportRepo := commentreport.NewRepository(db, "sql/queries/business")

	roots, err := adjacencyRepo.GetRootCommentsByPostID(context.Background(), 1)
	if err != nil {
		log.Fatalf("get root comments: %v", err)
	}
	fmt.Println("adjacency root comments:")
	for _, root := range roots {
		fmt.Printf("- id=%d body=%s\n", root.ID, root.Body)
	}

	tree, err := adjacencyRepo.GetPostCommentTree(context.Background(), 1)
	if err != nil {
		log.Fatalf("get adjacency post comment tree: %v", err)
	}
	fmt.Println("adjacency full tree:")
	printNodes(tree, 0)

	closureSubtree, err := closureRepo.GetCommentSubtree(context.Background(), 1)
	if err != nil {
		log.Fatalf("get closure subtree: %v", err)
	}
	fmt.Println("closure subtree from comment 1:")
	for _, comment := range closureSubtree {
		fmt.Printf("- id=%d body=%s\n", comment.ID, comment.Body)
	}

	ancestors, err := closureRepo.GetAncestorChain(context.Background(), 4)
	if err != nil {
		log.Fatalf("get closure ancestors: %v", err)
	}
	fmt.Println("closure ancestor chain for comment 4:")
	for _, comment := range ancestors {
		fmt.Printf("- id=%d body=%s\n", comment.ID, comment.Body)
	}

	closureTree, err := closureRepo.GetPostCommentTree(context.Background(), 1)
	if err != nil {
		log.Fatalf("get closure post comment tree: %v", err)
	}
	fmt.Println("closure full tree:")
	printNodes(closureTree, 0)

	pathSubtree, err := pathRepo.GetCommentSubtree(context.Background(), 1)
	if err != nil {
		log.Fatalf("get path subtree: %v", err)
	}
	fmt.Println("path subtree from comment 1:")
	for _, comment := range pathSubtree {
		fmt.Printf("- id=%d body=%s\n", comment.ID, comment.Body)
	}

	pathAncestors, err := pathRepo.GetAncestorChain(context.Background(), 4)
	if err != nil {
		log.Fatalf("get path ancestors: %v", err)
	}
	fmt.Println("path ancestor chain for comment 4:")
	for _, comment := range pathAncestors {
		fmt.Printf("- id=%d body=%s\n", comment.ID, comment.Body)
	}

	pathTree, err := pathRepo.GetPostCommentTree(context.Background(), 1)
	if err != nil {
		log.Fatalf("get path post comment tree: %v", err)
	}
	fmt.Println("path full tree:")
	printNodes(pathTree, 0)

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

func printNodes(nodes []*commenttree.CommentNode, depth int) {
	prefix := ""
	for i := 0; i < depth; i++ {
		prefix += "  "
	}

	for _, node := range nodes {
		fmt.Printf("%s- id=%d body=%s\n", prefix, node.Comment.ID, node.Comment.Body)
		printNodes(node.Children, depth+1)
	}
}
