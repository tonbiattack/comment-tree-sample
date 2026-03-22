-- ============================================================
-- 閉包テーブル方式: 祖先チェーン取得
-- ============================================================
-- 目的:
--   指定したコメント（descendant_id = ?）のルートから自分自身までの
--   祖先コメントを深さ順（ルートが先頭）に返す。
--   自分自身（depth=0 の自己参照行）も結果に含む。
--
-- 想定ユースケース:
--   - パンくずリスト（どのスレッドへの返信か経路を表示する）
--   - 返信元スレッドのプレビュー表示
--   例: "comment1 > comment2 > comment4" のような階層表示
--
-- 主要な出力項目:
--   id, post_id, parent_id, body, created_at
--
-- 実装方針:
--   comment_closures テーブルで descendant_id = ? に絞ると、
--   そのコメントの全祖先（自己参照含む）の ancestor_id が得られる。
--   それを comments テーブルに JOIN して祖先コメントの本体を取得する。
--
--   ORDER BY cc.depth DESC の理由:
--     depth はその「祖先から対象コメントまでの距離」を表す。
--     - depth が大きい = ルートに近い祖先（遠い祖先）
--     - depth が小さい = 対象コメントに近い祖先
--     DESC にすることで「ルート（depth最大）→ ... → 自分（depth=0）」の順で並ぶ。
--
--   Materialized Path 方式の ancestor_chain.sql との比較:
--     - 閉包テーブル: descendant_id での直接検索のみ（JOIN 1 回）
--     - Materialized Path: path 文字列の LIKE マッチが必要
--
-- パラメータ:
--   ? = 対象コメントの id（= comment_closures.descendant_id）(BIGINT)
--
-- インデックス利用:
--   comment_closures: idx_comment_closures_descendant_id (descendant_id) で
--   descendant_id フィルタが効率よく機能する。
--
-- 実行例（descendant_id = 4 の場合）:
--   comment_closures から descendant_id=4 の行:
--     (1,4,2), (2,4,1), (4,4,0)
--   ancestor_id で comments に JOIN した結果:
--     id=1(depth=2), id=2(depth=1), id=4(depth=0)
--   ORDER BY cc.depth DESC → depth: 2→1→0 の順 = id: 1, 2, 4
-- ============================================================
SELECT
    c.id,
    c.post_id,
    c.parent_id,
    c.body,
    c.created_at
FROM comment_closures cc
-- ancestor_id で comments テーブルを JOIN し、祖先コメントの本体を取得する
INNER JOIN comments c
    ON c.id = cc.ancestor_id
-- descendant_id = ? で「この行の祖先にあたる closure 行」に絞る
WHERE cc.descendant_id = ?
-- depth DESC: ルート（depth が大きい = 距離が遠い）を先頭に並べる
-- created_at, id はタイブレーク用
ORDER BY cc.depth DESC, c.created_at, c.id;
