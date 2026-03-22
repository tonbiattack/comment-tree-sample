-- ============================================================
-- ビジネスロジック: 投稿コメント統計スナップショット
-- ============================================================
-- 目的:
--   指定した投稿（p.id = ?）のコメント全体に関する統計情報を1行で返す。
--
-- 想定ユースケース:
--   - 投稿一覧ページの「コメント4件・最終更新2026-01-01」などのサマリ表示
--   - ダッシュボードでの議論活発度の一覧表示
--
-- 主要な出力項目:
--   post_id, post_title, root_comment_count, total_comment_count,
--   max_depth, latest_comment_at
--
-- 実装方針:
--   posts LEFT JOIN comments で集計する。
--   LEFT JOIN を使う理由: コメントが1件もない投稿（PostC など）でも
--   NULL 行ではなく集計値 0 の行が返るようにするため。
--
--   各集計カラムの説明:
--
--   COUNT(DISTINCT CASE WHEN c.parent_id IS NULL THEN c.id END):
--     parent_id が NULL のコメントだけを COUNT する条件付き集計。
--     CASE WHEN で NULL を返した行は COUNT に含まれない（NULL を無視する COUNT の仕様）。
--     DISTINCT は複数コメントが同一 id を持つ異常ケース対策（実際はほぼ不要）。
--
--   COUNT(DISTINCT c.id):
--     全コメントの件数。LEFT JOIN で NULL 行が混入しても
--     c.id が NULL の行は COUNT されない。
--
--   COALESCE(MAX(c.depth), 0):
--     コメントが0件の場合 MAX(c.depth) は NULL になるため 0 に変換する。
--
--   MAX(c.created_at):
--     コメントが0件の場合 NULL になる。
--     Go 側で sql.NullTime として受け取りゼロ値として扱う。
--
-- パラメータ:
--   ? = post_id (BIGINT)
--
-- 実行例（post_id = 1 の場合）:
--   comments テーブル（post_id=1）: id=1(depth=0), id=2(depth=1), id=3(depth=1), id=4(depth=2)
--   root_comment_count = 1（id=1 のみ parent_id IS NULL）
--   total_comment_count = 4
--   max_depth = 2
--   latest_comment_at = '2026-01-01 10:04:00'（id=4 の created_at）
-- ============================================================
SELECT
    p.id AS post_id,
    p.title AS post_title,
    -- ルートコメント数: parent_id IS NULL のコメントのみカウント
    COUNT(DISTINCT CASE WHEN c.parent_id IS NULL THEN c.id END) AS root_comment_count,
    -- 総コメント数: NULL（コメントなし）は自動的に除外される
    COUNT(DISTINCT c.id) AS total_comment_count,
    -- 最大深さ: コメントなしの場合は NULL → 0 に変換
    COALESCE(MAX(c.depth), 0) AS max_depth,
    -- 最終コメント日時: コメントなしの場合は NULL（Go 側で NullTime として処理）
    MAX(c.created_at) AS latest_comment_at
FROM posts p
-- コメントが0件の投稿も結果に含めるために LEFT JOIN を使う
LEFT JOIN comments c
    ON c.post_id = p.id
WHERE p.id = ?
GROUP BY
    p.id,
    p.title;
