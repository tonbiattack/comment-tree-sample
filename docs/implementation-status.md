# 実装状況と今後の提案

## 1. 現在の実装状況

このリポジトリは、コメントツリーの **Adjacency List / Materialized Path / Closure Table** の 3方式を比較できる状態です。

### 実装済み

#### MySQL / Docker

- `docker-compose.yml`
  - `mysql:8.4` で起動可能
  - ホスト側ポートは `.env` の `MYSQL_HOST_PORT` で変更可能
- `scripts/bootstrap-db.ps1`
  - `.env` 未作成時の自動生成
  - ポート競合時に `33306..33320` の範囲で空きポートへ自動退避
  - コンテナ起動待機

#### SQL / スキーマ

- `docker/mysql-init/01-schema.sql`
  - `posts`
  - `comments`
  - カラムコメント、テーブルコメントを追加済み
- `docker/mysql-init/02-data.sql`
  - サンプルデータ投入済み
- `sql/all.sql`
  - テスト再投入用の一括 SQL
- `sql/queries/root_comments.sql`
  - ルートコメント取得
- `sql/queries/comment_subtree.sql`
  - 再帰 CTE によるサブツリー取得
- `sql/queries/post_comment_tree.sql`
  - 投稿全体のコメントツリー取得

#### Go 実装

- `internal/commenttree/comment.go`
  - `Comment`
  - `CommentNode`
- `internal/commenttree/repository.go`
  - `CreateComment`
  - `GetRootCommentsByPostID`
  - `GetCommentSubtree`
  - `GetPostCommentTree`
- `internal/commenttree/tree.go`
  - 平坦なコメント一覧から木構造へ変換
- `internal/pathtree/repository.go`
  - Materialized Path によるコメント作成
  - path prefix によるサブツリー取得
  - 祖先チェーン取得
  - 投稿全体ツリー取得
- `internal/closuretree/repository.go`
  - Closure Table によるコメント作成
  - 閉包テーブルによるサブツリー取得
  - 祖先チェーン取得
  - 投稿全体ツリー取得
- `cmd/samplefetch/main.go`
  - 3方式の取得比較サンプル

#### テスト

- `internal/commenttree/repository_test.go`
  - コメント作成
  - 存在しない親コメントの異常系
  - ルートコメント取得
  - サブツリー取得
  - 投稿全体ツリー構築
- `internal/pathtree/repository_test.go`
  - path / depth 更新
  - path prefix によるサブツリー取得
  - 祖先チェーン取得
  - 投稿全体ツリー構築
- `internal/closuretree/repository_test.go`
  - closure 行追加
  - 閉包テーブルによるサブツリー取得
  - 祖先チェーン取得
  - 投稿全体ツリー構築
- `test/testdb/mysql.go`
  - 実 DB 接続
  - `sql/all.sql` による DB リセット

### 実行確認済み

- `go test ./...`
- `go run .\cmd\samplefetch`
- `docker compose` による MySQL 起動

## 2. まだ未実装のもの

現状は「DB + Repository + 実 DB テスト + サンプル取得」までです。次は未対応です。

- Gin API
- Cobra CLI
- Clean Architecture 全体構成
- ドメインサービス / usecase 層
- コメント更新
- コメント削除
- 深さ制限付き取得
- reply_count 集計
- ページング
- ソフトデリート
- 認可 / 認証
- migration 管理ツール導入

## 3. このサンプルプロジェクトに追加したほうが良い機能

優先度順に追加候補を整理します。

### 優先度高

#### 1. Gin API

このサンプルは「Go からどう使うか」を見せる価値が大きいため、HTTP API を追加したほうがよいです。

最低限あるとよい API:

- `POST /posts/:postID/comments`
- `GET /posts/:postID/comments/root`
- `GET /posts/:postID/comments/tree`
- `GET /comments/:commentID/tree`

#### 2. 入力バリデーション

今は `CreateComment` で最低限の親存在確認だけです。以下を追加したほうがよいです。

- `body` 空文字禁止
- `body` 長さ制限
- `post_id` 存在確認
- `created_at` の扱いを DB or アプリのどちらで決めるか統一

#### 3. コメント削除機能

ツリー構造では削除仕様が重要です。少なくとも次のどちらかを実装したほうがよいです。

- 単体削除
- サブツリー削除

仕様としては、どちらを採用するかを明記すべきです。

#### 4. 深さ制限付き取得

学習用サンプルとして価値が高いです。

例:

- `max_depth = 1` なら子まで
- `max_depth = 2` なら孫まで

再帰 CTE の条件追加例を示せるため、教材として強くなります。

#### 5. 投稿全体ツリーの JSON 出力サンプル

今は標準出力への表示のみです。API を見据えるなら JSON 化した例があると分かりやすいです。

## 4. 追加すると学習価値が上がる機能

#### 1. reply_count 集計

各コメントに対して返信数を付けると、一覧系 SQL の題材として使いやすくなります。

#### 2. ソート条件の切り替え

現在は `created_at, id` 固定です。以下を切り替えられるようにすると実用寄りになります。

- 作成日時昇順
- 作成日時降順

#### 3. 複数投稿サンプルデータ

今は `PostA` 1件のみです。複数投稿にすると、`post_id` による分離や誤取得防止の検証がしやすくなります。

#### 4. Phase 2 / Phase 3 の比較資料

このプロジェクトを `spec2.md` の方向に広げるなら、将来的に以下を追加すると比較教材として強くなります。

- Materialized Path 版
- Closure Table 版

## 5. 現時点の問題点

### 1. Clean Architecture にはまだなっていない

現状は repository 中心の最小実装で、`usecase` や `interface` 層は未整備です。

このため、サンプルとしては分かりやすい一方で、実務寄りの構成例としてはまだ不足があります。

### 2. テストが単一 DB を作り直す前提

各テストは `sql/all.sql` を使って DB を毎回初期化します。これは分かりやすいですが、次の制約があります。

- `t.Parallel()` とは相性が悪い
- 同時にサンプル実行すると競合する
- テスト件数が増えると遅くなりやすい

今後は次のいずれかを検討するとよいです。

- テストごとに専用 DB を使う
- トランザクションロールバック方式に切り替える

### 3. `CreateComment` の責務が最小限

現在の異常系は「親コメントが存在しない」だけです。以下は未チェックです。

- 存在しない `post_id`
- 空本文
- 長すぎる本文
- 親コメントの循環防止の考慮

なお、現スキーマでは `parent_id` は既存コメントを指す必要があるため基本的な参照整合性はありますが、アプリケーション仕様としての検証は薄いです。

### 4. SQL の責務分割はまだ最小

検索 SQL は外出しできていますが、用途別 SQL の拡充はまだです。

今後追加するとよい SQL:

- `post_comment_tree_limited_depth.sql`
- `comment_reply_count_summary.sql`
- `post_comment_flat_timeline.sql`

### 5. 取得サンプルが CLI 風表示のみ

`cmd/samplefetch` は確認用途としては十分ですが、再利用性は低めです。

今後は次を追加したほうがよいです。

- JSON 出力
- 引数で `post_id`, `comment_id` を指定
- エラー時の終了コード整理

## 6. 次にやるとよい実装順

おすすめ順は次のとおりです。

1. Gin API を追加する
2. 入力バリデーションと異常系テストを増やす
3. コメント削除仕様を決めて実装する
4. 深さ制限付き取得を追加する
5. 複数投稿データと追加検索 SQL を増やす
6. 必要なら Materialized Path / Closure Table に拡張する

## 7. 現状のまとめ

現在は、コメントツリーの基礎を学ぶための **MySQL + Go の最小サンプル** としては成立しています。

特に次の点はすでに確認できる状態です。

- 隣接リストでのツリー表現
- 再帰 CTE での subtree 取得
- 投稿全体のツリー取得
- Go 側での木構造組み立て
- 実 DB を使った統合テスト

一方で、実務寄りサンプルとして完成度を上げるには、API 層、入力バリデーション、削除仕様、テスト分離戦略の追加が必要です。
