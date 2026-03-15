INSERT INTO posts (id, title, body, created_at)
SELECT 1, 'PostA', 'Sample post for comment tree', '2026-01-01 10:00:00'
WHERE NOT EXISTS (
    SELECT 1 FROM posts WHERE id = 1
);

INSERT INTO posts (id, title, body, created_at)
SELECT 2, 'PostB', 'Unanswered root thread sample', '2026-01-01 10:30:00'
WHERE NOT EXISTS (
    SELECT 1 FROM posts WHERE id = 2
);

INSERT INTO posts (id, title, body, created_at)
SELECT 3, 'PostC', 'No comment post sample', '2026-01-01 12:00:00'
WHERE NOT EXISTS (
    SELECT 1 FROM posts WHERE id = 3
);

INSERT INTO comments (id, post_id, parent_id, path, depth, body, created_at)
SELECT 1, 1, NULL, '/1/', 0, 'comment1', '2026-01-01 10:01:00'
WHERE NOT EXISTS (
    SELECT 1 FROM comments WHERE id = 1
);

INSERT INTO comments (id, post_id, parent_id, path, depth, body, created_at)
SELECT 2, 1, 1, '/1/2/', 1, 'comment2', '2026-01-01 10:02:00'
WHERE NOT EXISTS (
    SELECT 1 FROM comments WHERE id = 2
);

INSERT INTO comments (id, post_id, parent_id, path, depth, body, created_at)
SELECT 3, 1, 1, '/1/3/', 1, 'comment3', '2026-01-01 10:03:00'
WHERE NOT EXISTS (
    SELECT 1 FROM comments WHERE id = 3
);

INSERT INTO comments (id, post_id, parent_id, path, depth, body, created_at)
SELECT 4, 1, 2, '/1/2/4/', 2, 'comment4', '2026-01-01 10:04:00'
WHERE NOT EXISTS (
    SELECT 1 FROM comments WHERE id = 4
);

INSERT INTO comments (id, post_id, parent_id, path, depth, body, created_at)
SELECT 5, 2, NULL, '/5/', 0, 'comment5', '2026-01-01 11:00:00'
WHERE NOT EXISTS (
    SELECT 1 FROM comments WHERE id = 5
);

INSERT INTO comment_closures (ancestor_id, descendant_id, depth)
SELECT 1, 1, 0
WHERE NOT EXISTS (
    SELECT 1 FROM comment_closures WHERE ancestor_id = 1 AND descendant_id = 1
);

INSERT INTO comment_closures (ancestor_id, descendant_id, depth)
SELECT 1, 2, 1
WHERE NOT EXISTS (
    SELECT 1 FROM comment_closures WHERE ancestor_id = 1 AND descendant_id = 2
);

INSERT INTO comment_closures (ancestor_id, descendant_id, depth)
SELECT 1, 3, 1
WHERE NOT EXISTS (
    SELECT 1 FROM comment_closures WHERE ancestor_id = 1 AND descendant_id = 3
);

INSERT INTO comment_closures (ancestor_id, descendant_id, depth)
SELECT 1, 4, 2
WHERE NOT EXISTS (
    SELECT 1 FROM comment_closures WHERE ancestor_id = 1 AND descendant_id = 4
);

INSERT INTO comment_closures (ancestor_id, descendant_id, depth)
SELECT 2, 2, 0
WHERE NOT EXISTS (
    SELECT 1 FROM comment_closures WHERE ancestor_id = 2 AND descendant_id = 2
);

INSERT INTO comment_closures (ancestor_id, descendant_id, depth)
SELECT 2, 4, 1
WHERE NOT EXISTS (
    SELECT 1 FROM comment_closures WHERE ancestor_id = 2 AND descendant_id = 4
);

INSERT INTO comment_closures (ancestor_id, descendant_id, depth)
SELECT 3, 3, 0
WHERE NOT EXISTS (
    SELECT 1 FROM comment_closures WHERE ancestor_id = 3 AND descendant_id = 3
);

INSERT INTO comment_closures (ancestor_id, descendant_id, depth)
SELECT 4, 4, 0
WHERE NOT EXISTS (
    SELECT 1 FROM comment_closures WHERE ancestor_id = 4 AND descendant_id = 4
);

INSERT INTO comment_closures (ancestor_id, descendant_id, depth)
SELECT 5, 5, 0
WHERE NOT EXISTS (
    SELECT 1 FROM comment_closures WHERE ancestor_id = 5 AND descendant_id = 5
);
