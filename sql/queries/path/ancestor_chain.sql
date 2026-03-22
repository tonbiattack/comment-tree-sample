-- ============================================================
-- Materialized Path 方式: 祖先チェーン取得
-- ============================================================
-- 目的:
--   指定したコメント（target.id = ?）のルートから自分自身までの
--   祖先コメントを深さ順（ルートが先頭）に返す。
--   自分自身も結果に含む。
--
-- 想定ユースケース:
--   - パンくずリスト表示（例: "comment1 > comment2 > comment4"）
--   - 返信元スレッドのプレビュー表示
--
-- 主要な出力項目:
--   id, post_id, parent_id, body, created_at
--
-- 実装方針:
--   Materialized Path では「c が target の祖先である」ことを
--   「target.path が c.path で始まる」という前方一致で表現できる。
--
--   例: target.path = '/1/2/4/'
--     c.path = '/1/'    → target.path LIKE '/1/%'    → true  → c はコメント1（祖先）
--     c.path = '/1/2/'  → target.path LIKE '/1/2/%'  → true  → c はコメント2（祖先）
--     c.path = '/1/2/4/'→ target.path LIKE '/1/2/4/%'→ true  → c はコメント4（自分自身）
--     c.path = '/1/3/'  → target.path LIKE '/1/3/%'  → false → c はコメント3（兄弟、無関係）
--
--   ON target.path LIKE CONCAT(c.path, '%') の意味:
--     「c の path が target の path のプレフィックスである」
--     = 「c は target の祖先（または自分自身）である」
--
--   ORDER BY c.depth:
--     depth=0（ルート）が先頭になり、depth が増えるにつれ target に近づく順で並ぶ。
--
--   閉包テーブル方式の ancestor_chain.sql との比較:
--     - 閉包テーブル: descendant_id の直接検索のみ（JOIN 1 回、固定コスト）
--     - Materialized Path: LIKE を使った自己 JOIN（セルフ JOIN + 文字列比較）
--                          インデックス効率はパスの長さと一致数に依存する
--
-- パラメータ:
--   ? = 対象コメントの id (BIGINT)
--
-- インデックス利用:
--   target 側: PRIMARY KEY (id) で target コメントを 1 件取得
--   c 側: path LIKE CONCAT(c.path, '%') は c.path のプレフィックスで target をフィルタするが、
--         こちらは全件スキャンになりやすい。行数が少ない場合は問題ない。
--
-- 実行例（id = 4, target.path = '/1/2/4/' の場合）:
--   CONCAT(c.path, '%') のパターンごとの評価:
--     c.path='/1/'    → '/1/2/4/' LIKE '/1/%'    → マッチ（c=コメント1）
--     c.path='/1/2/'  → '/1/2/4/' LIKE '/1/2/%'  → マッチ（c=コメント2）
--     c.path='/1/2/4/'→ '/1/2/4/' LIKE '/1/2/4/%'→ マッチ（c=コメント4=自分）
--     c.path='/1/3/'  → '/1/2/4/' LIKE '/1/3/%'  → 不一致
--   ORDER BY c.depth → 0, 1, 2 = id順: 1, 2, 4
-- ============================================================
-- 対象コメントの path を使って、祖先に当たるコメントだけを拾う
SELECT
    c.id,
    c.post_id,
    c.parent_id,
    c.body,
    c.created_at
FROM comments target
-- 「target の path が c の path で始まる」= 「c は target の祖先（または自分）」
INNER JOIN comments c
    ON target.path LIKE CONCAT(c.path, '%')
-- target コメントを id で特定する
WHERE target.id = ?
-- depth 昇順: ルートコメント（depth=0）が先頭になる
ORDER BY c.depth, c.created_at, c.id;
