CREATE TABLE IF NOT EXISTS category_translations (
    id SERIAL PRIMARY KEY,
    category_id INT,
    language_id INT,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);