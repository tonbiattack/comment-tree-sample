-- ============================================================
-- ビジネスロジック: ルートスレッドサマリ一覧
-- ============================================================
-- 目的:
--   指定した投稿（root.post_id = ?）配下の各ルートコメントについて、
--   直属返信数・全子孫数・最大深さ・最終返信日時を集計して返す。
--
-- 想定ユースケース:
--   - 投稿詳細画面でスレッドごとの活発度を表示する
--   - モデレーション画面で返信が多いスレッドを上位表示する
--   例: "comment1（返信2件・子孫3件・最終更新2026-01-01）"
--
-- 主要な出力項目:
--   root_comment_id, root_body, direct_reply_count,
--   descendant_count, max_depth, latest_reply_at
--
-- 実装方針:
--   comments(root) を起点に 3 つの LEFT JOIN を組み合わせる:
--
--   [1] LEFT JOIN comment_closures cc ON cc.ancestor_id = root.id
--     → ルートコメントの全子孫（自己参照 depth=0 含む）の closure 行を取得
--     → ここから descendant_count と max_depth を集計する
--
--   [2] LEFT JOIN comments descendant ON descendant.id = cc.descendant_id
--     → closure の descendant_id からコメント本体を取得
--     → latest_reply_at (MAX(descendant.created_at)) の計算に使う
--
--   [3] LEFT JOIN comments direct_child ON direct_child.parent_id = root.id
--     → ルートへの直属返信（parent_id = root.id）を取得
--     → direct_reply_count の計算に使う
--
--   なぜ direct_reply_count を closure から取れないのか:
--     closure テーブルには depth=1 の行もあるため
--     COUNT(CASE WHEN cc.depth = 1) でも計算できるが、
--     comments テーブルから直接 parent_id で集計する方が
--     意図が明確で closure テーブルに依存しない。
--
--   各集計カラムの説明:
--
--   COUNT(DISTINCT direct_child.id):
--     direct_child は LEFT JOIN のため、直属返信なし → NULL → COUNT は 0。
--
--   COUNT(DISTINCT CASE WHEN cc.depth > 0 THEN cc.descendant_id END):
--     depth=0 は自己参照行（ルートコメント自身）なので除外する。
--     depth > 0 の子孫コメントのみをカウントする。
--
--   COALESCE(MAX(cc.depth), 0):
--     子孫なし（LEFT JOIN で NULL）の場合 MAX は NULL → 0 に変換。
--
--   MAX(descendant.created_at):
--     ルートへの返信が0件の場合 NULL（Go 側で NullTime として処理）。
--
-- パラメータ:
--   ? = post_id (BIGINT)
--
-- 実行例（post_id = 1 の場合）:
--   ルートコメントは id=1 のみ
--   direct_child: id=2, id=3 の2件 → direct_reply_count=2
--   closures: (1,2,1), (1,3,1), (1,4,2) → descendant_count=3, max_depth=2
--   latest_reply_at: MAX(created_at of id=2,3,4) = '2026-01-01 10:04:00'
-- ============================================================
SELECT
    root.id AS root_comment_id,
    root.body AS root_body,
    -- 直属返信数: parent_id = root.id のコメント件数
    COUNT(DISTINCT direct_child.id) AS direct_reply_count,
    -- 全子孫数: 自己参照行（depth=0）を除いた closure の descendant 件数
    COUNT(DISTINCT CASE WHEN cc.depth > 0 THEN cc.descendant_id END) AS descendant_count,
    -- 最大深さ: 子孫なしの場合は NULL → 0 に変換
    COALESCE(MAX(cc.depth), 0) AS max_depth,
    -- 最終返信日時: 全子孫の中で最も新しい created_at
    MAX(descendant.created_at) AS latest_reply_at
FROM comments root
-- [1] 全子孫（自己参照含む）の closure 行を取得
LEFT JOIN comment_closures cc
    ON cc.ancestor_id = root.id
-- [2] closure の descendant_id からコメント本体を取得（latest_reply_at 用）
LEFT JOIN comments descendant
    ON descendant.id = cc.descendant_id
-- [3] 直属返信コメントを取得（direct_reply_count 用）
LEFT JOIN comments direct_child
    ON direct_child.parent_id = root.id
-- ルートコメント（parent_id IS NULL）のみを対象にする
WHERE root.post_id = ?
  AND root.parent_id IS NULL
GROUP BY
    root.id,
    root.body
-- 最終返信が新しい順に並べる（返信なしは NULL → リスト末尾）
ORDER BY
    latest_reply_at DESC,
    root.id;
