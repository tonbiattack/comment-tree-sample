# ツリー表現の比較

このリポジトリでは、コメントツリーを次の 3 方式で表現しています。

- 隣接リスト
- Materialized Path
- 閉包テーブル

同じ題材を 3 方式で並べることで、更新コストと検索コストの違いを比較できます。

## 1. 各方式の概要

### 隣接リスト

`comments.parent_id` で親子関係を表現する最も基本的な方式です。

- 親を 1 つだけ持つ
- ルートは `parent_id IS NULL`
- subtree 取得では再帰 CTE が必要

このリポジトリでの実装:

- SQL: `sql/queries/*.sql`
- Go: `internal/commenttree`

### Materialized Path

`comments.path` に祖先経路を文字列で保持する方式です。

例:

- `comment1`: `/1/`
- `comment2`: `/1/2/`
- `comment4`: `/1/2/4/`

特徴:

- subtree は `LIKE '/1/2/%'` の prefix 検索で取得しやすい
- ancestor も path を使って取得しやすい
- ノード移動時は配下すべての path 更新が必要

このリポジトリでの実装:

- SQL: `sql/queries/path/*.sql`
- Go: `internal/pathtree`

### 閉包テーブル

`comment_closures` に祖先子孫の全関係を保持する方式です。

例:

- `(1, 1, 0)`
- `(1, 2, 1)`
- `(1, 4, 2)`
- `(2, 4, 1)`

特徴:

- subtree 取得が単純な JOIN でできる
- ancestor 取得も単純
- INSERT 時に closure 行の追加が必要

このリポジトリでの実装:

- SQL: `sql/queries/closure/*.sql`
- Go: `internal/closuretree`

## 2. 比較表

| 観点 | 隣接リスト | Materialized Path | 閉包テーブル |
| --- | --- | --- | --- |
| データの単純さ | 最も単純 | 中間 | 最も複雑 |
| INSERT のしやすさ | 高い | 高い | 中程度 |
| subtree 取得 | 再帰 CTE が必要 | prefix 検索で比較的簡単 | JOIN で簡単 |
| ancestor 取得 | 再帰 or 反復が必要 | path 利用で比較的簡単 | JOIN で簡単 |
| 移動操作 | 中程度 | 難しい | 難しい |
| 削除操作 | 比較的分かりやすい | 比較的分かりやすい | closure 整合が必要 |
| SQL の分かりやすさ | 高い | 高い | 中程度 |
| 検索性能の拡張性 | 中程度 | 中程度 | 高い |
| 学習しやすさ | 最も高い | 高い | 中程度 |

## 3. このリポジトリで見られるポイント

### 隣接リストで見たいポイント

- `parent_id` による基本的な親子表現
- MySQL 再帰 CTE
- Go 側での木構築

対象:

- `internal/commenttree/repository.go`
- `sql/queries/comment_subtree.sql`

### Materialized Path で見たいポイント

- `path` / `depth` を持たせた冗長化
- prefix 検索による subtree 取得
- path から ancestor を復元する考え方

対象:

- `internal/pathtree/repository.go`
- `sql/queries/path/comment_subtree.sql`
- `sql/queries/path/ancestor_chain.sql`

### 閉包テーブルで見たいポイント

- `comment_closures` の自己行と祖先継承行
- INSERT 時の closure 展開
- JOIN ベースの subtree / ancestor 取得

対象:

- `internal/closuretree/repository.go`
- `sql/queries/closure/comment_subtree.sql`
- `sql/queries/closure/ancestor_chain.sql`

## 4. どの方式をいつ使うか

### 隣接リストが向く場合

- まずツリー構造を学びたい
- 実装を最小限にしたい
- 再帰 CTE を学習したい

### Materialized Path が向く場合

- subtree をよく読む
- path ベースの検索が許容できる
- 更新時に path の再計算を受け入れられる

### 閉包テーブルが向く場合

- ancestor / descendant の検索が多い
- 読み取り性能を優先したい
- INSERT や移動時の更新コストを受け入れられる

## 5. このサンプルの見方

おすすめの読む順番:

1. 隣接リスト
2. Materialized Path
3. 閉包テーブル

この順番にすると、次の流れで理解しやすいです。

- まず最小構成を理解する
- 次に冗長な列で検索を楽にする考え方を学ぶ
- 最後に関係の事前展開で検索を強くする設計を見る

## 6. 現時点の注意点

このリポジトリは比較学習用の最小構成です。以下はまだ本格対応していません。

- ノード移動
- ソフトデリート
- 深さ制限付き取得
- 大量データでの実行計画比較
- API 層での 3 方式切り替え

そのため、現時点では「各方式の考え方と基本取得方法を比較する教材」として使うのが適切です。
