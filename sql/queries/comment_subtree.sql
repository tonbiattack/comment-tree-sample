-- ============================================================
-- 隣接リスト方式: サブツリー取得（再帰 CTE）
-- ============================================================
-- 目的:
--   指定したコメント（id = ?）を根として、その全子孫コメントを返す。
--   指定コメント自身も結果に含む。
--
-- 想定ユースケース:
--   「このコメントへの返信スレッドを全て展開して表示する」場面。
--   例: スレッドの一部分だけをフォールドイン/アウトする UI。
--
-- 主要な出力項目:
--   id, post_id, parent_id, body, created_at
--   (depth は CTE 内部で計算しているが外部 SELECT では除外している)
--
-- 実装方針:
--   WITH RECURSIVE（再帰 CTE）を使って隣接リストの親子関係を再帰的に展開する。
--   - アンカー部: 指定コメント1件を depth=0 として取得する。
--   - 再帰部: comments を comment_tree に JOIN し、
--             parent_id = ct.id （つまり「今まで取得した行の子」）を追加していく。
--             ct.depth + 1 で深さをインクリメントする。
--   - 終了条件: 子が存在しない行に達したとき自動的に再帰が終了する（UNION ALL の JOIN が 0 件）。
--   - ORDER BY depth, created_at, id で深さ優先・同深さは作成順に安定ソートする。
--
-- パラメータ:
--   ? = 起点となるコメントの id (BIGINT)
--
-- 注意事項:
--   MySQL の再帰 CTE はデフォルトで最大 1000 回の再帰深度制限がある（cte_max_recursion_depth）。
--   非常に深いツリーでは SET cte_max_recursion_depth = N; が必要になる場合がある。
--
-- 実行例（コメント1を起点とした場合）:
--   アンカー: id=1, depth=0
--   1回目の再帰: id=2(parent=1), depth=1  /  id=3(parent=1), depth=1
--   2回目の再帰: id=4(parent=2), depth=2
--   結果順: 1, 2, 3, 4 （depth昇順→created_at昇順）
-- ============================================================
WITH RECURSIVE comment_tree AS (
    -- [アンカー部] 再帰の起点: 指定コメント1件を depth=0 で取得する
    SELECT
        id,
        post_id,
        parent_id,
        body,
        created_at,
        0 AS depth
    FROM comments
    WHERE id = ?

    UNION ALL

    -- [再帰部] CTE に既に追加された行の「直接の子コメント」を追加していく。
    -- c.parent_id = ct.id が「ct の行が c の親である」という結合条件。
    -- ct.depth + 1 で各コメントの深さを計算する。
    SELECT
        c.id,
        c.post_id,
        c.parent_id,
        c.body,
        c.created_at,
        ct.depth + 1 AS depth
    FROM comments c
    INNER JOIN comment_tree ct
        ON c.parent_id = ct.id
)
-- depth は内部計算用なので外部 SELECT には含めない
-- （Go 側の queryComments のスキャン列と一致させるため）
-- CTE で集めたコメントをそのまま返す
SELECT
    id,
    post_id,
    parent_id,
    body,
    created_at
FROM comment_tree
-- depth 昇順 → 同深さは created_at 昇順 → さらに id 昇順で完全に安定させる
ORDER BY depth, created_at, id;
