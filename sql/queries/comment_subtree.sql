-- 目的: 指定したコメントを根として子孫を再帰取得する。
-- 想定ユースケース: 返信スレッドの一部分だけを展開表示する。
-- 主要な出力項目: コメントID、投稿ID、親コメントID、本文、作成日時。
-- 実装方針: 再帰CTEで深さを計算し、UI比較が安定するよう depth, created_at, id で並べる。
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
