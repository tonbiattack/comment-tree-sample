# private-comment-tree-sample

MySQL 8 を Docker で起動できる最小構成を追加しています。ホスト側ポートは競合を避けるため `3306` 固定ではなく、既定で `33306` を使います。

3方式の比較資料は `docs/tree-model-comparison.md` を参照してください。

## 起動方法

```powershell
Copy-Item .env.example .env
docker compose up -d mysql
```

または、待機込みの補助スクリプトを使います。

```powershell
.\scripts\bootstrap-db.ps1
```

このスクリプトは、`MYSQL_HOST_PORT` が使用中なら `33306..33320` の範囲で空きポートを探して `.env` を更新します。

初回起動時に `docker/mysql-init/01-schema.sql` と `docker/mysql-init/02-data.sql` が実行され、以下のサンプルデータが投入されます。

- `posts`: `PostA`
- `comments`: `1 -> (2, 3)`、`2 -> 4`

## ポート変更

他の DB コンテナやローカル MySQL と衝突する場合は `.env` の `MYSQL_HOST_PORT` を変更してください。

```env
MYSQL_HOST_PORT=33307
```

接続例:

```text
Host: 127.0.0.1
Port: 33306
Database: comment_tree
User: comment_user
Password: comment_pass
```

## 再作成

データボリュームを消して作り直す場合:

```powershell
.\scripts\bootstrap-db.ps1 -Recreate
```

## SQL 配置

- 一括投入用: `sql/all.sql`
- Docker 初期化用: `docker/mysql-init/01-schema.sql`
- Docker 初期データ用: `docker/mysql-init/02-data.sql`
- 隣接リスト検索 SQL: `sql/queries/*.sql`
- 業務サマリ SQL: `sql/queries/business/*.sql`
- Materialized Path 検索 SQL: `sql/queries/path/*.sql`
- 閉包テーブル検索 SQL: `sql/queries/closure/*.sql`

## 実装済みのツリー表現

### 隣接リスト

- テーブル: `comments`
- Go 実装: `internal/commenttree`
- 機能:
  - コメント作成
  - ルートコメント取得
  - 再帰 CTE によるサブツリー取得
  - 投稿全体ツリー取得

### 閉包テーブル

- テーブル: `comment_closures`
- Go 実装: `internal/closuretree`
- 機能:
  - コメント作成時の closure 行追加
  - 閉包テーブルによるサブツリー取得
  - 祖先チェーン取得
  - 投稿全体ツリー取得

### Materialized Path

- テーブル: `comments.path`, `comments.depth`
- Go 実装: `internal/pathtree`
- 機能:
  - コメント作成時の path / depth 更新
  - path prefix によるサブツリー取得
  - 祖先チェーン取得
  - 投稿全体ツリー取得

### 業務サマリ

- Go 実装: `internal/commentreport`
- 機能:
  - 投稿ごとのコメント状況サマリ
  - ルートスレッドごとの活動サマリ

## Go テスト

MySQL コンテナ起動後に実行します。テストは実 DB を使い、毎回 `sql/all.sql` でスキーマとサンプルデータを再投入します。

```powershell
$env:MYSQL_HOST_PORT="33308"
$env:MYSQL_DATABASE="comment_tree"
$env:MYSQL_USER="comment_user"
$env:MYSQL_PASSWORD="comment_pass"
go test ./...
```

## 取得サンプル

隣接リスト、閉包テーブル、Materialized Path の 3方式で取得例を確認できるサンプルを `cmd/samplefetch` に用意しています。

```powershell
$env:MYSQL_HOST_PORT="33308"
$env:MYSQL_DATABASE="comment_tree"
$env:MYSQL_USER="comment_user"
$env:MYSQL_PASSWORD="comment_pass"
go run .\cmd\samplefetch
```

出力例:

```text
adjacency root comments:
- id=1 body=comment1
adjacency full tree:
- id=1 body=comment1
  - id=2 body=comment2
    - id=4 body=comment4
  - id=3 body=comment3
closure subtree from comment 1:
- id=1 body=comment1
- id=2 body=comment2
- id=3 body=comment3
- id=4 body=comment4
closure ancestor chain for comment 4:
- id=1 body=comment1
- id=2 body=comment2
- id=4 body=comment4
path subtree from comment 1:
- id=1 body=comment1
- id=2 body=comment2
- id=3 body=comment3
- id=4 body=comment4
path ancestor chain for comment 4:
- id=1 body=comment1
- id=2 body=comment2
- id=4 body=comment4
business snapshot: post_id=1 title=PostA roots=1 total=4 max_depth=2 latest=2026-01-01T10:04:00Z
business root thread summaries:
- root_id=1 direct=2 descendants=3 max_depth=2 latest=2026-01-01T10:04:00Z
```
