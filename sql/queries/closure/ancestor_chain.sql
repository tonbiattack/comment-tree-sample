-- 目的: 閉包テーブルを使って指定コメントの祖先チェーンを取得する。
-- 想定ユースケース: パンくずや返信元の経路表示に使う。
-- 主要な出力項目: コメントID、投稿ID、親コメントID、本文、作成日時。
-- 実装方針: descendant_id を起点に ancestor 側へ JOIN し、depth 降順でルートから対象コメントの順に並べる。
SELECT
    c.id,
    c.post_id,
    c.parent_id,
    c.body,
    c.created_at
FROM comment_closures cc
INNER JOIN comments c
    ON c.id = cc.ancestor_id
WHERE cc.descendant_id = ?
ORDER BY cc.depth DESC, c.created_at, c.id;
