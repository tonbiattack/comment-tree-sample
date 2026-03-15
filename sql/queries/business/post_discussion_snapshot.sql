-- 目的: 投稿単位のコメント状況サマリを返す。
-- 想定ユースケース: 投稿一覧やダッシュボードで、議論の活発さを一覧表示する。
-- 主要な出力項目: 投稿ID、タイトル、ルートコメント数、総コメント数、最大深さ、最終コメント日時。
-- 実装方針: comments の depth を集計し、ルート件数と総件数を同時に算出する。
SELECT
    p.id AS post_id,
    p.title AS post_title,
    COUNT(DISTINCT CASE WHEN c.parent_id IS NULL THEN c.id END) AS root_comment_count,
    COUNT(DISTINCT c.id) AS total_comment_count,
    COALESCE(MAX(c.depth), 0) AS max_depth,
    MAX(c.created_at) AS latest_comment_at
FROM posts p
LEFT JOIN comments c
    ON c.post_id = p.id
WHERE p.id = ?
GROUP BY
    p.id,
    p.title;
