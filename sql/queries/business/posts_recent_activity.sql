-- ============================================================
-- ビジネスロジック: 最近の活動順 投稿一覧
-- ============================================================
-- 目的:
--   全投稿をコメントの最終更新日時の降順で返す。
--   コメントが0件の投稿もリストに含め、リストの末尾に表示する。
--
-- 想定ユースケース:
--   - 投稿一覧画面で「最近コメントがあった投稿を上位に表示する」場面
--   - 管理画面でアクティブな議論を把握する
--   例: PostB（最終更新11:00）→ PostA（最終更新10:04）→ PostC（コメントなし）
--
-- 主要な出力項目:
--   post_id, post_title, total_comment_count, latest_comment_at
--
-- 実装方針:
--   posts LEFT JOIN comments で全投稿を対象にしつつ、
--   コメントなしの投稿（PostC）も除外せずに返す。
--
--   ORDER BY の3段階ソートの説明:
--
--   [1] latest_comment_at IS NULL:
--     IS NULL は MySQL では 0 (false) または 1 (true) を返す。
--     ASC ソート（デフォルト）なので:
--       - IS NULL = 0（NULL でない = コメントあり）→ 先頭グループ
--       - IS NULL = 1（NULL = コメントなし）→ 末尾グループ
--     これにより「コメントなし投稿」が確実にリスト末尾に来る。
--
--   [2] latest_comment_at DESC:
--     コメントありの投稿グループ内で、最終コメントが新しい順に並べる。
--
--   [3] p.id:
--     latest_comment_at が同一の場合の安定ソート用タイブレーク。
--
--   INNER JOIN ではなく LEFT JOIN を使う理由:
--     INNER JOIN だとコメントが0件の投稿がクエリ結果から消えてしまう。
--     LEFT JOIN なら c.* は NULL になるが p.* の行は残る。
--     COUNT(c.id) は NULL を無視するため 0 として集計される。
--
-- パラメータ:
--   なし（全投稿対象）
--
-- 実行例（初期データの場合）:
--   PostB(id=2): latest_comment_at='2026-01-01 11:00:00', total=1
--   PostA(id=1): latest_comment_at='2026-01-01 10:04:00', total=4
--   PostC(id=3): latest_comment_at=NULL, total=0
--   ORDER BY: PostB → PostA → PostC（NULL は末尾）
-- ============================================================
SELECT
    p.id AS post_id,
    p.title AS post_title,
    -- コメント件数: NULL（コメントなし）は自動的に 0 として集計される
    COUNT(c.id) AS total_comment_count,
    -- 最終コメント日時: コメントなしの場合 NULL（Go 側で NullTime として処理）
    MAX(c.created_at) AS latest_comment_at
FROM posts p
-- コメントが0件の投稿も除外しないために LEFT JOIN を使う
LEFT JOIN comments c
    ON c.post_id = p.id
GROUP BY
    p.id,
    p.title
ORDER BY
    -- [1] NULL（コメントなし）を末尾に送る: IS NULL = 1 (true) は ASC で後ろになる
    latest_comment_at IS NULL,
    -- [2] コメントありの投稿は最終更新が新しい順
    latest_comment_at DESC,
    -- [3] タイブレーク: 投稿 ID 昇順
    p.id;
