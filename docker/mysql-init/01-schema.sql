CREATE TABLE IF NOT EXISTS posts (
    id BIGINT AUTO_INCREMENT PRIMARY KEY COMMENT '投稿ID',
    title VARCHAR(255) NOT NULL COMMENT '投稿タイトル',
    body TEXT COMMENT '投稿本文',
    created_at DATETIME NOT NULL COMMENT '投稿作成日時',
    INDEX idx_posts_created_at (created_at)
) ENGINE=InnoDB COMMENT='投稿を管理するテーブル';

CREATE TABLE IF NOT EXISTS comments (
    id BIGINT AUTO_INCREMENT PRIMARY KEY COMMENT 'コメントID',
    post_id BIGINT NOT NULL COMMENT '所属する投稿ID',
    parent_id BIGINT NULL COMMENT '親コメントID。NULL はルートコメント',
    path VARCHAR(1024) NOT NULL COMMENT 'Materialized Path 形式の経路。例: /1/2/',
    depth INT NOT NULL COMMENT 'ルートからの深さ。ルートは 0',
    body TEXT NOT NULL COMMENT 'コメント本文',
    created_at DATETIME NOT NULL COMMENT 'コメント作成日時',
    INDEX idx_comments_post_id (post_id),
    INDEX idx_comments_parent_id (parent_id),
    INDEX idx_comments_post_path (post_id, path(255)),
    INDEX idx_comments_post_depth_created_at (post_id, depth, created_at),
    INDEX idx_comments_post_parent_created_at (post_id, parent_id, created_at),
    UNIQUE KEY uq_comments_id_post_id (id, post_id),
    CONSTRAINT fk_comments_post
        FOREIGN KEY (post_id) REFERENCES posts(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_comments_parent_same_post
        FOREIGN KEY (parent_id, post_id) REFERENCES comments(id, post_id)
        ON DELETE CASCADE
) ENGINE=InnoDB COMMENT='投稿に紐づくコメントの隣接リスト構造を管理するテーブル';

CREATE TABLE IF NOT EXISTS comment_closures (
    ancestor_id BIGINT NOT NULL COMMENT '祖先コメントID。自己行も含む',
    descendant_id BIGINT NOT NULL COMMENT '子孫コメントID。自己行も含む',
    depth INT NOT NULL COMMENT '祖先から子孫までの距離。自己行は 0',
    PRIMARY KEY (ancestor_id, descendant_id),
    INDEX idx_comment_closures_descendant_id (descendant_id),
    INDEX idx_comment_closures_ancestor_depth (ancestor_id, depth, descendant_id),
    CONSTRAINT fk_comment_closures_ancestor
        FOREIGN KEY (ancestor_id) REFERENCES comments(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_comment_closures_descendant
        FOREIGN KEY (descendant_id) REFERENCES comments(id)
        ON DELETE CASCADE
) ENGINE=InnoDB COMMENT='コメント間の祖先子孫関係を事前展開して保持する閉包テーブル';
