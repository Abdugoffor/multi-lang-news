CREATE INDEX CONCURRENTLY IF NOT EXISTS languages_is_active_id_idx
    ON languages (is_active, id DESC);
