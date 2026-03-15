-- 目的: 投稿配下の各ルートスレッドについて活動サマリを返す。
-- 想定ユースケース: モデレーション画面や投稿詳細で、活発なスレッド順に並べる。
-- 主要な出力項目: ルートコメントID、本文、直属返信数、子孫総数、最大深さ、最終返信日時。
-- 実装方針: 閉包テーブルで子孫数と深さを集計し、直属返信数は comments.parent_id から別集計する。
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
ORDER BY
    latest_reply_at DESC,
    root.id;
