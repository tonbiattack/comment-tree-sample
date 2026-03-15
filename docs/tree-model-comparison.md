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

### 現代の実務でのおすすめ

結論だけ先に書くと、現代でも最初の選択肢としては **隣接リスト** が最もおすすめです。

理由:

- スキーマが最も単純
- 更新処理が分かりやすい
- RDBMS 側の再帰 CTE が一般的になっており、以前より subtree 取得を書きやすい
- 将来、必要になったら Materialized Path や 閉包テーブルへ拡張しやすい

ただし、これは「まず始めるなら」の話です。読み取り要件が強い場合は次の判断が現実的です。

- **まずは隣接リスト**
  - 要件がまだ固まっていない
  - 更新処理を単純に保ちたい
  - 学習用、社内ツール、小中規模機能
- **subtree / ancestor の読み取り頻度が高いなら Materialized Path**
  - path 更新コストを受け入れられる
  - ノード移動が少ない
  - prefix 検索で十分に扱える
- **祖先子孫検索を強く最適化したいなら閉包テーブル**
  - 更新より読み取りを優先する
  - 集計や階層検索が多い
  - 実装と保守の複雑さを許容できる

このリポジトリでも、比較学習用としては 3 方式を並べていますが、実務で最初に採用するなら次のコメントになります。

- 第一候補: 隣接リスト
- 次点: Materialized Path
- 強い検索要件がある場合のみ: 閉包テーブル

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

## 5. 採用コメント

短く判断するなら次のコメントになります。

- **隣接リスト**: 現代でも最初に選びやすい、最もバランスの良い標準選択
- **Materialized Path**: 読み取りを少し強くしたいときの現実的な拡張案
- **閉包テーブル**: 高機能だが重い。強い検索要件があるときに採用を検討する方式

## 6. このサンプルの見方

おすすめの読む順番:

1. 隣接リスト
2. Materialized Path
3. 閉包テーブル

この順番にすると、次の流れで理解しやすいです。

- まず最小構成を理解する
- 次に冗長な列で検索を楽にする考え方を学ぶ
- 最後に関係の事前展開で検索を強くする設計を見る

## 7. 現時点の注意点

このリポジトリは比較学習用の最小構成です。以下はまだ本格対応していません。

- ノード移動
- ソフトデリート
- 深さ制限付き取得
- 大量データでの実行計画比較
- API 層での 3 方式切り替え

そのため、現時点では「各方式の考え方と基本取得方法を比較する教材」として使うのが適切です。
