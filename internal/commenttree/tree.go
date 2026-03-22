package commenttree

// BuildTree はフラットなコメントリストからツリー構造を組み立てます。
//
// データベースから取得した平坦なコメントリストを受け取り、
// 親子関係に基づいてネストした []*CommentNode を返します。
//
// アルゴリズムの概要:
//  1. 全コメントを ID → *CommentNode のマップに登録する
//  2. 各コメントについて親を探し、見つかれば親の Children に追加する
//  3. 親が存在しない（ParentID == nil、または親IDがマップにない）場合はルートとして扱う
//
// 注意: 親IDがマップに存在しない場合（親コメントが取得範囲外）は、
// そのコメントをルートとして返すフォールバック動作をします。
// これにより、サブツリー取得クエリで部分的なツリーが渡された場合でも
// パニックせず動作します。
func BuildTree(comments []Comment) []*CommentNode {
	// ID → ノードのマップを事前に確保（len で容量を指定してメモリ効率を高める）
	nodes := make(map[int64]*CommentNode, len(comments))
	// 元の順序を保持するためにIDのスライスを別途記録する
	// map のイテレーション順序は不定なため、order スライスで順序を保証する
	order := make([]int64, 0, len(comments))

	// パス1: 全コメントをノードとしてマップに登録する
	for _, comment := range comments {
		// ループ変数のアドレスを取らないようにコピーを作成する
		commentCopy := comment
		nodes[comment.ID] = &CommentNode{Comment: commentCopy}
		order = append(order, comment.ID)
	}

	// パス2: 親子関係を解決してツリーを組み立てる
	roots := make([]*CommentNode, 0)
	for _, id := range order {
		node := nodes[id]
		if node.Comment.ParentID == nil {
			// ParentID が nil = ルートコメント
			roots = append(roots, node)
			continue
		}

		parent, ok := nodes[*node.Comment.ParentID]
		if !ok {
			// 親コメントが取得範囲外の場合はルートとして扱う（フォールバック）
			roots = append(roots, node)
			continue
		}

		parent.Children = append(parent.Children, node)
	}

	return roots
}
