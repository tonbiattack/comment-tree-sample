-- ============================================================
-- 隣接リスト方式: ルートコメント取得
-- ============================================================
-- 目的:
--   指定した投稿（post_id = ?）に属するルートコメント（トップレベルのコメント）を
--   作成日時順に返す。
--
-- 想定ユースケース:
--   投稿詳細画面でスレッドの起点となるコメント一覧を表示する。
--   例: Reddit のトップレベルコメント一覧、掲示板のスレッド一覧。
--
-- 主要な出力項目:
--   id, post_id, parent_id (常に NULL), body, created_at
--
-- 実装方針:
--   - parent_id IS NULL でルートコメントのみを絞り込む。
--   - ORDER BY created_at, id で表示順を完全に固定する。
--     created_at だけでは同一時刻のコメントの順序が不定になるため id を第2キーに使う。
--
-- パラメータ:
--   ? = post_id (BIGINT)
--
-- インデックス利用:
--   idx_comments_post_id (post_id) でまず投稿を絞り、
--   その後 parent_id IS NULL でフィルタリングする。
--   行数が多い場合は idx_comments_post_parent_created_at (post_id, parent_id, created_at)
--   が有効に使われる。
-- ============================================================
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
