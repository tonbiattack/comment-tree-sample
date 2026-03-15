-- 目的: Materialized Path を使って指定コメント配下の子孫を取得する。
-- 想定ユースケース: prefix 検索でスレッド部分木を取得する。
-- 主要な出力項目: コメントID、投稿ID、親コメントID、本文、作成日時。
-- 実装方針: path の前方一致で subtree を取得し、depth と作成順で安定した表示順を保つ。
SELECT
    id,
    post_id,
    parent_id,
    body,
    created_at
FROM comments
WHERE path LIKE CONCAT(?, '%')
ORDER BY depth, created_at, id;
