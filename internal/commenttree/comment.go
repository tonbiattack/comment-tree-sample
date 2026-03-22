// Package commenttree は、隣接リスト方式でコメントツリーを管理するパッケージです。
//
// このパッケージには、コメントのデータ型定義・ツリー構築ロジック・
// データベースアクセスリポジトリが含まれます。
//
// 隣接リスト方式の特徴:
//   - 各コメントが親コメントのIDのみを持つシンプルな設計
//   - スキーマが単純で実装コストが低い
//   - サブツリーの取得には再帰CTE（WITH RECURSIVE）が必要
//   - path/depth カラムはこの方式では使用しない（DEFAULT値のまま）
package commenttree

import "time"

// Comment はコメント1件を表す構造体です。
//
// 3つのツリー表現方式（隣接リスト・閉包テーブル・Materialized Path）で
// 共通して使用されます。各方式によって利用するフィールドが異なります。
type Comment struct {
	// ID はコメントの一意識別子（データベースの自動採番値）
	ID int64
	// PostID はこのコメントが属する投稿のID（外部キー）
	PostID int64
	// ParentID は親コメントのID。NULL の場合はルートコメント（トップレベル）を示す。
	// ポインタ型にすることで NULL（ルートコメント）を表現する。
	ParentID *int64
	// Path は Materialized Path 方式で使用するパス文字列。
	// 例: "/1/2/4/" のように、ルートから自分自身までのIDをスラッシュ区切りで格納する。
	// 隣接リスト方式では使用しないため DEFAULT 値（"/"）のまま。
	Path string
	// Depth はルートコメントからの深さ（0始まり）。
	// Materialized Path・閉包テーブル方式で使用する。
	// 隣接リスト方式では使用しないため DEFAULT 値（0）のまま。
	Depth int
	// Body はコメントの本文テキスト
	Body string
	// CreatedAt はコメントの作成日時
	CreatedAt time.Time
}

// CommentNode はツリー構造でコメントを表現するノードです。
//
// BuildTree 関数によってフラットなコメントリストから組み立てられ、
// 各ノードが自分の子コメントへの参照を持つことでツリーを形成します。
type CommentNode struct {
	// Comment はこのノードが保持するコメントデータ
	Comment Comment
	// Children はこのコメントへの直接の返信（子コメント）のリスト。
	// 末端コメント（返信なし）では空スライス。
	Children []*CommentNode
}
