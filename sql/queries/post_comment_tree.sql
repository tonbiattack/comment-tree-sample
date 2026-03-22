-- ============================================================
-- 隣接リスト方式: 投稿全体のコメントツリー取得（再帰 CTE）
-- ============================================================
-- 目的:
--   指定した投稿（post_id = ?）に属する全コメントを取得する。
--   Go 側の BuildTree に渡すことでメモリ上でツリー構造に変換する。
--
-- 想定ユースケース:
--   投稿詳細画面でコメントツリー全体を一度に描画する場面。
--
-- 主要な出力項目:
--   id, post_id, parent_id, body, created_at
--
-- 実装方針:
--   comment_subtree.sql の「起点を1コメントに固定」する版との違い:
--     アンカー部で post_id = ? AND parent_id IS NULL により
--     「指定投稿のルートコメント全て」を起点にしている。
--   これにより、1投稿に複数のルートコメントが存在する場合でも全てを展開できる。
--
--   ORDER BY depth, created_at, id:
--     Go 側 BuildTree は入力順序に依存しないが、
--     深さ優先に近い順序で渡すことで LLM/デバッグ時の可読性が向上する。
--
-- パラメータ:
--   ? = post_id (BIGINT)
--
-- インデックス利用:
--   アンカー部: idx_comments_post_id (post_id) + parent_id IS NULL フィルタ
--   再帰部: idx_comments_parent_id (parent_id) で子を効率よく探す
--
-- 実行例（投稿1の場合）:
--   アンカー: id=1, depth=0 （post_id=1 のルートコメントは1件）
--   1回目: id=2(depth=1), id=3(depth=1)
--   2回目: id=4(depth=2)
--   結果順: 1, 2, 3, 4
-- ============================================================
WITH RECURSIVE comment_tree AS (
    -- [アンカー部] 指定投稿のルートコメント（parent_id IS NULL）を全て起点にする
    SELECT
        id,
        post_id,
        parent_id,
        body,
        created_at,
        0 AS depth
    FROM comments
    WHERE post_id = ?
      AND parent_id IS NULL

    UNION ALL

    -- [再帰部] CTE に追加済みの行の直接の子コメントを追加していく
    SELECT
        c.id,
        c.post_id,
        c.parent_id,
        c.body,
        c.created_at,
        ct.depth + 1 AS depth
    FROM comments c
    INNER JOIN comment_tree ct
        ON c.parent_id = ct.id
)
SELECT
    id,
    post_id,
    parent_id,
    body,
    created_at
FROM comment_tree
ORDER BY depth, created_at, id;
