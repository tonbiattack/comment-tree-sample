package commenttree

func BuildTree(comments []Comment) []*CommentNode {
	nodes := make(map[int64]*CommentNode, len(comments))
	order := make([]int64, 0, len(comments))

	for _, comment := range comments {
		commentCopy := comment
		nodes[comment.ID] = &CommentNode{Comment: commentCopy}
		order = append(order, comment.ID)
	}

	roots := make([]*CommentNode, 0)
	for _, id := range order {
		node := nodes[id]
		if node.Comment.ParentID == nil {
			roots = append(roots, node)
			continue
		}

		parent, ok := nodes[*node.Comment.ParentID]
		if !ok {
			roots = append(roots, node)
			continue
		}

		parent.Children = append(parent.Children, node)
	}

	return roots
}
