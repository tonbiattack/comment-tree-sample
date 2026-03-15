-- 目的: 指定した投稿に属するコメントツリー全体を取得する。
-- 想定ユースケース: 投稿詳細画面で全コメントツリーを描画する。
-- 主要な出力項目: コメントID、投稿ID、親コメントID、本文、作成日時。
-- 実装方針: ルートコメントを起点に再帰CTEで全件展開し、深さ順と作成順で安定ソートする。
WITH RECURSIVE comment_tree AS (
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
