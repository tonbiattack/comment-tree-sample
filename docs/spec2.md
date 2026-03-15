では、**ツリー構造を段階的に学べる最強構成**としてまとめます。
AIに渡しやすいように、題材・段階・成果物を整理します。

---

# プロジェクト全体コンセプト

1つの題材を使って、ツリー構造の表現方法を段階的に学ぶ。
題材は **コメントスレッド** に統一する。

理由

* イメージしやすい
* 隣接リストと相性が良い
* Materialized Path / Closure Table にも拡張しやすい
* Go + MySQL で実装しやすい

---

# おすすめプロジェクト名

総合プロジェクト名

* **go-tree-structure-lab**
* **go-tree-model-sample**
* **go-comment-tree-lab**
* **tree-model-playground**
* **sql-tree-pattern-sample**

おすすめはこれです。

**go-tree-structure-lab**

理由

* コメント専用に閉じない
* ツリー構造の学習用だと分かりやすい
* GitHub名としても使いやすい

---

# 最強構成

## Phase 1. 隣接リスト

最初にやるべき構成。

### モデル

```text
comments
- id
- post_id
- parent_id
- body
- created_at
```

### 学べること

* ツリー構造の基本
* parent_id による親子関係
* 再帰CTE
* Goでの木構築

### 実装機能

* コメント作成
* ルートコメント一覧取得
* あるコメント配下の subtree 取得
* 投稿全体のコメントツリー取得

### この段階の目的

「ツリーはまず parent_id で表せる」を理解すること。

---

## Phase 2. Materialized Path

次に学ぶと理解が深まる構成。

### モデル

```text
comments
- id
- post_id
- parent_id
- path
- depth
- body
- created_at
```

例

```text
1        path=/1/
2        path=/1/2/
4        path=/1/2/4/
```

### 学べること

* pathベースの階層表現
* prefix検索
* depth管理
* subtree取得のしやすさ

### 実装機能

* コメント作成時に path を生成
* subtree取得
* 祖先取得
* depth制限付き取得

### この段階の目的

「取得を楽にするために冗長な情報を持つ」発想を学ぶこと。

---

## Phase 3. Closure Table

最後にやる上級構成。

### モデル

```text
comments
- id
- post_id
- parent_id
- body
- created_at

comment_closures
- ancestor_id
- descendant_id
- depth
```

例

```text
ancestor descendant depth
1        1          0
1        2          1
1        4          2
2        2          0
2        4          1
4        4          0
```

### 学べること

* 祖先子孫関係の事前展開
* subtree/ancestor検索高速化
* INSERT時の closure 更新
* ツリーDB設計の実務寄り発想

### 実装機能

* コメント追加時に closure 行追加
* subtree取得
* ancestor chain取得
* depth指定取得

### この段階の目的

「更新コストを払って検索を強くする」設計を学ぶこと。

---

# 学習順

おすすめ順はこれです。

1. **Adjacency List**
2. **Materialized Path**
3. **Closure Table**

この順が良い理由

* まず最小構成を理解できる
* 次に冗長列で読み取り改善を学べる
* 最後に検索最適化の発想まで到達できる

---

# 1つのリポジトリでの構成案

```text
go-tree-structure-lab/
  README.md
  docs/
    adjacency-list.md
    materialized-path.md
    closure-table.md
  schema/
    adjacency_list.sql
    materialized_path.sql
    closure_table.sql
  internal/
    adjacency/
    path/
    closure/
  test/
```

これがかなり良いです。
**同じ題材を別モデルで比較できる**からです。

---

# READMEに入れるべき比較観点

各モデルについてこれを並べると強いです。

* データ構造
* INSERTしやすさ
* subtree取得しやすさ
* ancestor取得しやすさ
* 移動操作の難しさ
* 削除の難しさ
* SQLの分かりやすさ
* 実務での採用しやすさ

---

# 学習用としての最終ゴール

このプロジェクトのゴールは、単にツリーを作ることではなく、次を比較できるようになることです。

* どのモデルが一番単純か
* どのモデルが検索しやすいか
* どのモデルが更新しやすいか
* どの場面でどの設計を採用するべきか

---

# 一番おすすめの進め方

最初は **Phase 1だけを完成** させるのが良いです。
その後に Phase 2, 3 を追加する形が自然です。

なので最初のAI向け仕様としては、

**go-tree-structure-lab の Phase 1: Adjacency List版**

を作らせるのが一番良いです。

---

# すぐ使える題材名

* コメントスレッド
* フォルダツリー
* カテゴリ階層
* 組織階層

この中ではやはり
**コメントスレッド** が一番おすすめです。

---

次に必要なら、
**go-tree-structure-lab の完全仕様書（Phase 1〜3対応）** をそのままAIに渡せる形で作れます。
