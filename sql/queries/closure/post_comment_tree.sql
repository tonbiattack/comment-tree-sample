-- 目的: 閉包テーブルを使って指定投稿のコメントツリー全体を取得する。
-- 想定ユースケース: 投稿詳細画面で全コメントツリーを再帰 CTE なしで描画する。
-- 主要な出力項目: コメントID、投稿ID、親コメントID、本文、作成日時。
-- 実装方針: comments 単体を取得して Go 側で木構造化する前提で、親IDと作成順を使って親子が近接するよう並べる。
SELECT
    c.id,
    c.post_id,
    c.parent_id,
    c.body,
    c.created_at
FROM comments c
WHERE c.post_id = ?
ORDER BY
    COALESCE(c.parent_id, c.id),
    c.parent_id IS NULL DESC,
    c.created_at,
    c.id;
