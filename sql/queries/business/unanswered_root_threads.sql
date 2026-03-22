-- ============================================================
-- ビジネスロジック: 未返信ルートスレッド一覧
-- ============================================================
-- 目的:
--   指定した投稿（root.post_id = ?）配下のルートコメントのうち、
--   直属返信が0件のもの（未返信スレッド）を返す。
--
-- 想定ユースケース:
--   - モデレーション画面や運用ダッシュボードで「誰も返信していないスレッド」を検出する
--   - サポート系の投稿で、未対応の質問やリクエストを拾い上げる
--   例: "comment5（返信なし）"
--
-- 主要な出力項目:
--   root_comment_id, root_body, direct_reply_count (常に0),
--   descendant_count, max_depth, latest_reply_at
--
-- 実装方針:
--   root_thread_summaries.sql と全く同じ集計ロジックを使い、
--   最後に HAVING で直属返信が0件のものに絞る。
--
--   WHERE ではなく HAVING を使う理由:
--     direct_reply_count は GROUP BY 後の集計値（COUNT の結果）なので、
--     WHERE 句では参照できない。HAVING は GROUP BY 後のフィルタリングに使う。
--
--   COUNT(DISTINCT direct_child.id) = 0 の動作:
--     direct_child は LEFT JOIN のため、直属返信なしの場合 direct_child.id は NULL。
--     COUNT(DISTINCT NULL) = 0 となるため正しく機能する。
--
--   ORDER BY の違い（root_thread_summaries.sql との比較）:
--     root_thread_summaries.sql: latest_reply_at DESC（活発なスレッド順）
--     unanswered_root_threads.sql: root.created_at（古い未返信スレッドが先頭）
--     未返信一覧は「古いものから対応する」という運用を想定した並び順にしている。
--
-- パラメータ:
--   ? = post_id (BIGINT)
--
-- 実行例（post_id = 2 の場合）:
--   ルートコメントは id=5 のみ
--   direct_child: 0件 → direct_reply_count=0
--   HAVING COUNT(DISTINCT direct_child.id) = 0 → id=5 は条件を満たす
--   返却: root_comment_id=5, direct_reply_count=0, descendant_count=0
-- ============================================================
SELECT
    root.id AS root_comment_id,
    root.body AS root_body,
    -- 直属返信数: 未返信スレッドなので常に 0 になるが、型一致のために出力する
    COUNT(DISTINCT direct_child.id) AS direct_reply_count,
    -- 全子孫数: 自己参照行（depth=0）を除いた closure の descendant 件数
    COUNT(DISTINCT CASE WHEN cc.depth > 0 THEN cc.descendant_id END) AS descendant_count,
    -- 最大深さ: 子孫なしの場合は NULL → 0 に変換
    COALESCE(MAX(cc.depth), 0) AS max_depth,
    -- 最終返信日時: 未返信なので NULL（Go 側で NullTime として処理）
    MAX(descendant.created_at) AS latest_reply_at
FROM comments root
-- [1] 全子孫（自己参照含む）の closure 行を取得
LEFT JOIN comment_closures cc
    ON cc.ancestor_id = root.id
-- [2] closure の descendant_id からコメント本体を取得（latest_reply_at 用）
LEFT JOIN comments descendant
    ON descendant.id = cc.descendant_id
-- [3] 直属返信コメントを取得（HAVING での絞り込み用）
LEFT JOIN comments direct_child
    ON direct_child.parent_id = root.id
-- ルートコメント（parent_id IS NULL）のみを対象にする
WHERE root.post_id = ?
  AND root.parent_id IS NULL
GROUP BY
    root.id,
    root.body
-- HAVING で集計後のフィルタリング: 直属返信が0件のスレッドのみ
HAVING COUNT(DISTINCT direct_child.id) = 0
-- 古い未返信スレッドが先頭になるよう作成日時の昇順で並べる
ORDER BY
    root.created_at,
    root.id;
