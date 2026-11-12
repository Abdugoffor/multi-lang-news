CREATE TABLE IF NOT EXISTS news_translations (
    news_id BIGINT NOT NULL REFERENCES news(id) ON DELETE RESTRICT,
    language_id INT NOT NULL REFERENCES languages(id) ON DELETE RESTRICT,
    title VARCHAR(255) NOT NULL CHECK (btrim(title) <> ''),
    description TEXT NOT NULL CHECK (btrim(description) <> ''),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (news_id, language_id)
);

CREATE INDEX IF NOT EXISTS news_translations_language_id_idx ON news_translations(language_id);
