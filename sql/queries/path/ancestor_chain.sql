-- 目的: Materialized Path を使って指定コメントの祖先チェーンを取得する。
-- 想定ユースケース: パンくず表示や返信元の経路表示に使う。
-- 主要な出力項目: コメントID、投稿ID、親コメントID、本文、作成日時。
-- 実装方針: target.path の prefix に一致する path を祖先とみなし、depth 昇順でルートから自分まで返す。
SELECT
    c.id,
    c.post_id,
    c.parent_id,
    c.body,
    c.created_at
FROM comments target
INNER JOIN comments c
    ON target.path LIKE CONCAT(c.path, '%')
WHERE target.id = ?
ORDER BY c.depth, c.created_at, c.id;
