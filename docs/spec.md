AIに渡すことを前提に、**曖昧さを減らした仕様ドキュメント形式**で書きます。
（そのままプロンプトに入れられるレベル）

---

# サンプルプロジェクト名

候補

* **comment-tree-sample**
* **thread-tree-sample**
* **comment-hierarchy-sample**
* **go-comment-tree**
* **tree-structure-sample**

おすすめ

**go-comment-tree-sample**

理由

* Go + ツリー構造が分かる
* GitHubで検索しやすい

---

# プロジェクト仕様書

Comment Tree Sample Project

## 1. プロジェクト目的

コメントの返信構造をツリー構造として管理するサンプルプロジェクトを作成する。

このプロジェクトの目的は次のとおり。

* ツリー構造の基本的なデータモデルを理解する
* SQLの再帰CTEを利用した階層取得を学ぶ
* Goアプリケーションからツリー構造を扱う方法を示す
* 実務に近いサンプルとして利用できるようにする

対象DBは **MySQL 8以上** とする。

---

# 2. 概要

1つの投稿(Post)に対して複数のコメント(Comment)が存在する。

コメントは返信可能であり、
コメントはコメントを親に持つことができる。

これによりコメントはツリー構造になる。

例

```
Post
 └ Comment1
      ├ Comment2
      │    └ Comment4
      └ Comment3
```

---

# 3. データモデル

## posts

投稿テーブル

```sql
CREATE TABLE posts (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    body TEXT,
    created_at DATETIME NOT NULL
);
```

---

## comments

コメントテーブル

```sql
CREATE TABLE comments (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    post_id BIGINT NOT NULL,
    parent_id BIGINT NULL,
    body TEXT NOT NULL,
    created_at DATETIME NOT NULL,

    INDEX idx_post_id (post_id),
    INDEX idx_parent_id (parent_id)
);
```

### カラム説明

| column     | description |
| ---------- | ----------- |
| id         | コメントID      |
| post_id    | 投稿ID        |
| parent_id  | 親コメントID     |
| body       | コメント本文      |
| created_at | 作成日時        |

---

# 4. ツリー構造のルール

ツリー構造は次のルールで管理する。

1. `parent_id` が NULL のコメントは **ルートコメント**
2. `parent_id` が存在する場合、そのコメントの返信
3. 親コメントは同じ `post_id` に属する必要がある

---

# 5. 想定データ例

```
posts
1 PostA
```

```
comments

id post_id parent_id body
1 1 NULL     comment1
2 1 1        comment2
3 1 1        comment3
4 1 2        comment4
```

ツリー構造

```
comment1
 ├ comment2
 │   └ comment4
 └ comment3
```

---

# 6. 必須機能

このサンプルプロジェクトでは次の機能を実装する。

## 6.1 コメント作成

新しいコメントを作成する。

入力

```
post_id
parent_id (nullable)
body
```

処理

* parent_id が指定された場合
* そのコメントが存在するか確認

---

## 6.2 ルートコメント取得

特定の投稿のルートコメントを取得する。

SQL例

```sql
SELECT *
FROM comments
WHERE post_id = ?
AND parent_id IS NULL
ORDER BY created_at;
```

---

## 6.3 コメントツリー取得

特定のコメントをルートとして
その子孫コメントをすべて取得する。

MySQL 再帰CTEを利用する。

SQL例

```sql
WITH RECURSIVE comment_tree AS (

    SELECT
        id,
        post_id,
        parent_id,
        body,
        created_at,
        0 AS depth
    FROM comments
    WHERE id = ?

    UNION ALL

    SELECT
        c.id,
        c.post_id,
        c.parent_id,
        c.body,
        c.created_at,
        ct.depth + 1
    FROM comments c
    JOIN comment_tree ct
        ON c.parent_id = ct.id
)

SELECT *
FROM comment_tree
ORDER BY depth, created_at;
```

---

## 6.4 投稿全体のコメントツリー取得

1つの投稿に属するコメントツリーを取得する。

ルートコメントを起点として
全コメントを再帰で取得する。

---

# 7. Goアプリケーション仕様

Goで次の構造体を定義する。

```go
type Comment struct {
    ID        int64
    PostID    int64
    ParentID  *int64
    Body      string
    CreatedAt time.Time
}
```

---

## 7.1 ツリー構造表現

Go側では次の構造を使う。

```go
type CommentNode struct {
    Comment  Comment
    Children []*CommentNode
}
```

---

# 8. テスト

テストでは次を検証する。

1. コメント作成
2. ツリー取得
3. 深い階層コメント
4. 子コメントなしケース

---

# 9. テストデータ例

```
1
 ├ 2
 │  └ 4
 └ 3
```

---

# 10. 実装対象

AIに生成させる対象

* SQL
* Go Repository
* 再帰CTE
* ツリー構築ロジック
* テストコード

---

# 11. 拡張課題（Optional）

以下は追加課題として実装可能。

### 1 深さ制限

```
max_depth
```

---

### 2 コメント削除

削除方式

* soft delete
* subtree delete

---

### 3 コメント数集計

```
reply_count
```

---

もし希望があれば、次に

**AIが最も作りやすい完全仕様**

* API仕様
* ディレクトリ構造
* Go repository interface
* SQL migration

まで含めた
**AI生成用 完全プロンプト**を作ります。
（かなり強いサンプルプロジェクトになります）
