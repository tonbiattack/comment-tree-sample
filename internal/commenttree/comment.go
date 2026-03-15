package commenttree

import "time"

type Comment struct {
	ID        int64
	PostID    int64
	ParentID  *int64
	Path      string
	Depth     int
	Body      string
	CreatedAt time.Time
}

type CommentNode struct {
	Comment  Comment
	Children []*CommentNode
}
