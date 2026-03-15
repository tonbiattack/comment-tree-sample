-- 目的: Materialized Path を使って指定投稿のコメントツリー全体を取得する。
-- 想定ユースケース: 投稿詳細画面で全コメントツリーを path 順に描画する。
-- 主要な出力項目: コメントID、投稿ID、親コメントID、本文、作成日時。
-- 実装方針: 同一投稿を path 順で取得し、祖先の直後に子孫が並ぶ性質を利用して Go 側で木構造化する。
SELECT
    id,
    post_id,
    parent_id,
    body,
    created_at
FROM comments
WHERE post_id = ?
ORDER BY path, created_at, id;
