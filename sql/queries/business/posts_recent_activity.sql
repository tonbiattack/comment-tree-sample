-- 目的: 投稿一覧を最近のコメント活動順で返す。
-- 想定ユースケース: 投稿一覧画面や管理画面で、最近動いた投稿を上位に表示する。
-- 主要な出力項目: 投稿ID、タイトル、総コメント数、最終コメント日時。
-- 実装方針: posts を起点に comments を LEFT JOIN し、コメントがない投稿も残しつつ最新活動日時で降順に並べる。
SELECT
    p.id AS post_id,
    p.title AS post_title,
    COUNT(c.id) AS total_comment_count,
    MAX(c.created_at) AS latest_comment_at
FROM posts p
LEFT JOIN comments c
    ON c.post_id = p.id
GROUP BY
    p.id,
    p.title
ORDER BY
    latest_comment_at IS NULL,
    latest_comment_at DESC,
    p.id;
