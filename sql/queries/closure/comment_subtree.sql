-- 目的: 閉包テーブルを使って指定コメント配下の子孫を取得する。
-- 想定ユースケース: 再帰 CTE を使わずにスレッド部分木を高速取得する。
-- 主要な出力項目: コメントID、投稿ID、親コメントID、本文、作成日時。
-- 実装方針: comment_closures を ancestor_id で絞り、depth と作成順で表示順を固定する。
SELECT
    c.id,
    c.post_id,
    c.parent_id,
    c.body,
    c.created_at
FROM comment_closures cc
INNER JOIN comments c
    ON c.id = cc.descendant_id
WHERE cc.ancestor_id = ?
ORDER BY cc.depth, c.created_at, c.id;
