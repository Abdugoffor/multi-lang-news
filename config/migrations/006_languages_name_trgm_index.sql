CREATE INDEX CONCURRENTLY IF NOT EXISTS languages_name_trgm_idx
    ON languages USING GIN (name gin_trgm_ops);
