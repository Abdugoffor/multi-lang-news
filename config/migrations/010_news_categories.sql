CREATE TABLE IF NOT EXISTS news_categories (
    category_id INT NOT NULL REFERENCES categories(id) ON DELETE RESTRICT,
    news_id BIGINT NOT NULL REFERENCES news(id) ON DELETE RESTRICT,
    PRIMARY KEY (news_id, category_id)
);

CREATE INDEX IF NOT EXISTS news_categories_category_id_idx ON news_categories(category_id, news_id);
