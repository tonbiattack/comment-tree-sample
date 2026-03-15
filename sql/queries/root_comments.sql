-- 目的: 指定した投稿に属するルートコメント一覧を取得する。
-- 想定ユースケース: 投稿詳細画面で、トップレベルのコメントを表示する。
-- 主要な出力項目: コメントID、投稿ID、親コメントID、本文、作成日時。
-- 実装方針: parent_id IS NULL のみを対象にし、表示順が安定するよう created_at, id で明示ソートする。
SELECT
    id,
    post_id,
    parent_id,
    body,
    created_at
FROM comments
WHERE post_id = ?
  AND parent_id IS NULL
ORDER BY created_at, id;
