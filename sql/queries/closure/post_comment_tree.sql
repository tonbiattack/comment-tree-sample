-- ============================================================
-- 閉包テーブル方式: 投稿全体のコメントツリー取得
-- ============================================================
-- 目的:
--   指定した投稿（post_id = ?）に属する全コメントを取得する。
--   Go 側の BuildTree に渡すことでメモリ上でツリー構造に変換する。
--
-- 想定ユースケース:
--   投稿詳細画面でコメントツリー全体を一度に描画する場面（閉包テーブル方式）。
--
-- 主要な出力項目:
--   id, post_id, parent_id, body, created_at
--
-- 実装方針:
--   この クエリは comment_closures テーブルを使わず、comments テーブルのみを使う。
--   BuildTree（Go 側）に全コメントをフラットに渡して、メモリ上でツリーを組み立てる設計。
--
--   閉包テーブルを使わない理由:
--     投稿全体のツリーは closure テーブルを JOIN しても余計な集計が不要なため、
--     シンプルに comments だけを取得して Go 側で組み立てる方が効率的。
--
--   ORDER BY COALESCE(c.parent_id, c.id), c.parent_id IS NULL DESC, c.created_at, c.id:
--     BuildTree は入力順序に依存しないが、親コメントと子コメントが近接するよう並べることで
--     ツリーの論理的なまとまりがわかりやすくなる。
--
--     - COALESCE(c.parent_id, c.id):
--         parent_id が NULL（ルートコメント）の場合は自分の id をグループキーとして使う。
--         これにより「同じルートスレッドに属するコメント」が隣り合って並ぶ。
--     - c.parent_id IS NULL DESC:
--         同じ COALESCE 値を持つ場合、ルートコメント（IS NULL = true）を先に出す。
--         MySQL では TRUE=1 / FALSE=0 なので DESC にすると TRUE（ルート）が先頭になる。
--     - c.created_at, c.id: 最終的なタイブレーク
--
-- パラメータ:
--   ? = post_id (BIGINT)
--
-- インデックス利用:
--   idx_comments_post_id (post_id) で投稿を絞り込む。
-- ============================================================
SELECT
    c.id,
    c.post_id,
    c.parent_id,
    c.body,
    c.created_at
FROM comments c
WHERE c.post_id = ?
ORDER BY
    -- COALESCE(parent_id, id): ルートコメントは自分の id をキーにして、
    -- 同じスレッドの子コメントがルートの直後に並ぶようにする
    COALESCE(c.parent_id, c.id),
    -- parent_id IS NULL DESC: 同じグループ内ではルートコメントを先頭に置く
    c.parent_id IS NULL DESC,
    c.created_at,
    c.id;
