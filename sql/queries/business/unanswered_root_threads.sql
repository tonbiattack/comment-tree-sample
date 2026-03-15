-- 目的: 直属返信がまだ付いていないルートスレッド一覧を返す。
-- 想定ユースケース: モデレーション画面や運用ダッシュボードで、未対応スレッドを確認する。
-- 主要な出力項目: ルートコメントID、本文、直属返信数、子孫数、最大深さ、最終返信日時。
-- 実装方針: ルートコメントを起点に closure 集計を行い、HAVING で直属返信 0 件のものだけに絞る。
SELECT
    root.id AS root_comment_id,
    root.body AS root_body,
    COUNT(DISTINCT direct_child.id) AS direct_reply_count,
    COUNT(DISTINCT CASE WHEN cc.depth > 0 THEN cc.descendant_id END) AS descendant_count,
    COALESCE(MAX(cc.depth), 0) AS max_depth,
    MAX(descendant.created_at) AS latest_reply_at
FROM comments root
LEFT JOIN comment_closures cc
    ON cc.ancestor_id = root.id
LEFT JOIN comments descendant
    ON descendant.id = cc.descendant_id
LEFT JOIN comments direct_child
    ON direct_child.parent_id = root.id
WHERE root.post_id = ?
  AND root.parent_id IS NULL
GROUP BY
    root.id,
    root.body
HAVING COUNT(DISTINCT direct_child.id) = 0
ORDER BY
    root.created_at,
    root.id;
