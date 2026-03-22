-- ============================================================
-- 閉包テーブル方式: サブツリー取得
-- ============================================================
-- 目的:
--   指定したコメント（ancestor_id = ?）を根として、
--   その全子孫コメント（自分自身を含む）を返す。
--
-- 想定ユースケース:
--   再帰 CTE を使わずにスレッド部分木を高速に取得したい場面。
--   閉包テーブルへの事前投資により、読み取りは単純な JOIN で完結する。
--
-- 主要な出力項目:
--   id, post_id, parent_id, body, created_at
--
-- 実装方針:
--   comment_closures テーブルには「全ての祖先・子孫ペア」が事前に格納されている。
--   ancestor_id = ? で絞ることで、そのコメントの全子孫（自己参照 depth=0 を含む）の
--   descendant_id が得られる。それを comments テーブルに JOIN して本体を取得する。
--
--   隣接リスト方式の再帰 CTE との比較:
--     - 再帰 CTE: ツリーの深さ分だけ反復実行されるため、深いツリーでは遅くなる
--     - 閉包テーブル: 常に O(サブツリーのノード数) の 1 回の JOIN で完結する
--
-- パラメータ:
--   ? = 起点となるコメントの id（= comment_closures.ancestor_id）(BIGINT)
--
-- インデックス利用:
--   comment_closures: PRIMARY KEY (ancestor_id, descendant_id) で ancestor_id フィルタが効く
--   さらに idx_comment_closures_ancestor_depth (ancestor_id, depth, descendant_id) で
--   ORDER BY cc.depth のソートがインデックスカバーされる可能性がある。
--   comments: idx_comments_post_id または PRIMARY KEY で descendant_id への JOIN が効く。
--
-- 実行例（ancestor_id = 1 の場合）:
--   comment_closures から ancestor_id=1 の行:
--     (1,1,0), (1,2,1), (1,3,1), (1,4,2)
--   descendant_id で comments に JOIN した結果:
--     id=1(depth=0), id=2(depth=1), id=3(depth=1), id=4(depth=2)
--   ORDER BY cc.depth, c.created_at, c.id → 1, 2, 3, 4
-- ============================================================
-- 閉包テーブルから子孫IDを引き、その実体を comments から取る
SELECT
    c.id,
    c.post_id,
    c.parent_id,
    c.body,
    c.created_at
FROM comment_closures cc
-- descendant_id で comments テーブルを JOIN し、コメント本体を取得する
INNER JOIN comments c
    ON c.id = cc.descendant_id
-- ancestor_id = ? で「指定コメントの全子孫（自己参照含む）」に絞る
WHERE cc.ancestor_id = ?
-- depth 昇順（浅い階層から）→ 同深さは created_at → id で安定ソート
ORDER BY cc.depth, c.created_at, c.id;
