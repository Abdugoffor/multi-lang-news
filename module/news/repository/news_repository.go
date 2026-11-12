package news_repository

import (
	"context"
	"encoding/json"
	"fmt"

	news_dto "test/module/news/dto"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type NewsRepository interface {
	List(ctx context.Context, filter *news_dto.Filter) ([]news_dto.ListItem, error)
	HasMore(ctx context.Context, filter *news_dto.Filter, cursor int64, direction string) (bool, error)
	Show(ctx context.Context, id int64) (*news_dto.Response, error)
	Create(ctx context.Context, req *news_dto.Create) (*news_dto.Response, error)
	Update(ctx context.Context, id int64, req *news_dto.Update) (*news_dto.Response, error)
	Delete(ctx context.Context, id int64) error
}

type newsRepository struct {
	db *pgxpool.Pool
}

func NewNewsRepository(db *pgxpool.Pool) NewsRepository {
	return &newsRepository{db: db}
}

func (repository *newsRepository) List(ctx context.Context, filter *news_dto.Filter) ([]news_dto.ListItem, error) {
	condition := "TRUE"
	args := []any{filter.LanguageID}
	order := "DESC"

	if filter.Direction == news_dto.DirectionPrev {
		order = "ASC"
	}

	if filter.Cursor > 0 {
		args = append(args, filter.Cursor)
		condition = "n.id < $2::BIGINT"

		if filter.Direction == news_dto.DirectionPrev {
			condition = "n.id > $2::BIGINT"
		}
	}

	if filter.Title != "" {
		args = append(args, filter.Title)
		condition += fmt.Sprintf(" AND COALESCE(t.title, '') ILIKE '%%' || $%d || '%%'", len(args))
	}

	if filter.CategoryID > 0 {
		args = append(args, filter.CategoryID)
		condition += fmt.Sprintf(" AND EXISTS (SELECT 1 FROM news_categories nc WHERE nc.news_id = n.id AND nc.category_id = $%d)", len(args))
	}

	if filter.UserID > 0 {
		args = append(args, filter.UserID)
		condition += fmt.Sprintf(" AND n.user_id = $%d", len(args))
	}

	if filter.IsMain != nil {
		args = append(args, *filter.IsMain)
		condition += fmt.Sprintf(" AND n.is_main = $%d", len(args))
	}

	if filter.IsActive != nil {
		args = append(args, *filter.IsActive)
		condition += fmt.Sprintf(" AND n.is_active = $%d", len(args))
	}

	args = append(args, filter.Limit+1)

	query := fmt.Sprintf(`
		SELECT n.id, n.is_main, n.is_active, n.view_count, n.photo, n.video, n.user_id,
			ARRAY(SELECT nc.category_id::BIGINT FROM news_categories nc WHERE nc.news_id = n.id ORDER BY nc.category_id),
			COALESCE(t.title, ''), COALESCE(t.description, ''), n.created_at, n.updated_at
		FROM news n
		LEFT JOIN LATERAL (
			SELECT nt.title, nt.description
			FROM news_translations nt
			WHERE nt.news_id = n.id
			ORDER BY (nt.language_id = $1::BIGINT) DESC, nt.language_id ASC
			LIMIT 1
		) t ON TRUE
		WHERE %s
		ORDER BY n.id %s
		LIMIT $%d
	`, condition, order, len(args))

	rows, err := repository.db.Query(ctx, query, args...)
	{
		if err != nil {
			return nil, err
		}
	}

	defer rows.Close()

	result := make([]news_dto.ListItem, 0, filter.Limit+1)

	for rows.Next() {
		var res news_dto.ListItem

		err := rows.Scan(
			&res.ID,
			&res.IsMain,
			&res.IsActive,
			&res.ViewCount,
			&res.Photo,
			&res.Video,
			&res.UserID,
			&res.CategoryIDs,
			&res.Title,
			&res.Description,
			&res.CreatedAt,
			&res.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		result = append(result, res)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func (repository *newsRepository) HasMore(ctx context.Context, filter *news_dto.Filter, cursor int64, direction string) (bool, error) {
	condition := "n.id < $2::BIGINT"
	args := []any{filter.LanguageID, cursor}

	if direction == news_dto.DirectionPrev {
		condition = "n.id > $2::BIGINT"
	}

	if filter.Title != "" {
		args = append(args, filter.Title)
		condition += fmt.Sprintf(" AND COALESCE(t.title, '') ILIKE '%%' || $%d || '%%'", len(args))
	}

	if filter.CategoryID > 0 {
		args = append(args, filter.CategoryID)
		condition += fmt.Sprintf(" AND EXISTS (SELECT 1 FROM news_categories nc WHERE nc.news_id = n.id AND nc.category_id = $%d)", len(args))
	}

	if filter.UserID > 0 {
		args = append(args, filter.UserID)
		condition += fmt.Sprintf(" AND n.user_id = $%d", len(args))
	}

	if filter.IsMain != nil {
		args = append(args, *filter.IsMain)
		condition += fmt.Sprintf(" AND n.is_main = $%d", len(args))
	}

	if filter.IsActive != nil {
		args = append(args, *filter.IsActive)
		condition += fmt.Sprintf(" AND n.is_active = $%d", len(args))
	}

	query := fmt.Sprintf(`
		SELECT EXISTS (
			SELECT 1 FROM news n
			LEFT JOIN LATERAL (
				SELECT nt.title, nt.description
				FROM news_translations nt
				WHERE nt.news_id = n.id
				ORDER BY (nt.language_id = $1::BIGINT) DESC, nt.language_id ASC
				LIMIT 1
			) t ON TRUE
			WHERE %s
		)
	`, condition)

	var exists bool

	err := repository.db.QueryRow(ctx, query, args...).Scan(&exists)
	{
		if err != nil {
			return false, err
		}
	}

	return exists, nil
}

func (repository *newsRepository) Show(ctx context.Context, id int64) (*news_dto.Response, error) {
	var res news_dto.Response

	err := repository.db.QueryRow(ctx, `
		SELECT n.id, n.is_main, n.is_active, n.view_count, n.photo, n.video, n.user_id,
			n.created_at, n.updated_at,
			ARRAY(SELECT nc.category_id::BIGINT FROM news_categories nc WHERE nc.news_id = n.id ORDER BY nc.category_id),
			COALESCE((
				SELECT jsonb_agg(to_jsonb(t) ORDER BY t.language_id)
				FROM news_translations t WHERE t.news_id = n.id
			), '[]'::jsonb)
		FROM news n WHERE n.id = $1
	`, id).Scan(
		&res.ID,
		&res.IsMain,
		&res.IsActive,
		&res.ViewCount,
		&res.Photo,
		&res.Video,
		&res.UserID,
		&res.CreatedAt,
		&res.UpdatedAt,
		&res.CategoryIDs,
		&res.Translations,
	)

	if err != nil {
		return nil, err
	}

	return &res, nil
}

func (repository *newsRepository) Create(ctx context.Context, req *news_dto.Create) (*news_dto.Response, error) {
	translations, err := json.Marshal(req.Translations)
	{
		if err != nil {
			return nil, err
		}
	}

	tx, err := repository.db.Begin(ctx)
	{
		if err != nil {
			return nil, err
		}
	}

	defer tx.Rollback(context.WithoutCancel(ctx))

	var res news_dto.Response

	err = tx.QueryRow(ctx, `
		INSERT INTO news (is_main, is_active, photo, video, user_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, is_main, is_active, view_count, photo, video, user_id, created_at, updated_at
	`, req.IsMain, req.IsActive, req.Photo, req.Video, req.UserID).Scan(
		&res.ID,
		&res.IsMain,
		&res.IsActive,
		&res.ViewCount,
		&res.Photo,
		&res.Video,
		&res.UserID,
		&res.CreatedAt,
		&res.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO news_translations (news_id, language_id, title, description)
		SELECT $1, t.language_id, t.title, t.description
		FROM jsonb_to_recordset(COALESCE(NULLIF($2::jsonb, 'null'::jsonb), '[]'::jsonb))
			AS t(language_id INT, title TEXT, description TEXT)
		ORDER BY t.language_id
	`, res.ID, translations)

	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO news_categories (news_id, category_id)
		SELECT $1, category_id FROM unnest($2::INT[]) AS c(category_id)
		ORDER BY category_id
	`, res.ID, req.CategoryIDs)

	if err != nil {
		return nil, err
	}

	err = tx.QueryRow(ctx, `
		SELECT ARRAY(
			SELECT nc.category_id::BIGINT FROM news_categories nc
			WHERE nc.news_id = $1 ORDER BY nc.category_id
		), COALESCE((
			SELECT jsonb_agg(to_jsonb(t) ORDER BY t.language_id)
			FROM news_translations t WHERE t.news_id = $1
		), '[]'::jsonb)
	`, res.ID).Scan(&res.CategoryIDs, &res.Translations)

	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &res, nil
}

func (repository *newsRepository) Update(ctx context.Context, id int64, req *news_dto.Update) (*news_dto.Response, error) {
	translations, err := json.Marshal(req.Translations)
	{
		if err != nil {
			return nil, err
		}
	}

	tx, err := repository.db.Begin(ctx)
	{
		if err != nil {
			return nil, err
		}
	}

	defer tx.Rollback(context.WithoutCancel(ctx))

	var res news_dto.Response

	err = tx.QueryRow(ctx, `
		UPDATE news
		SET is_main = $1, is_active = $2, photo = $3, video = $4, user_id = $5, updated_at = CURRENT_TIMESTAMP
		WHERE id = $6
		RETURNING id, is_main, is_active, view_count, photo, video, user_id, created_at, updated_at
	`, req.IsMain, req.IsActive, req.Photo, req.Video, req.UserID, id).Scan(
		&res.ID,
		&res.IsMain,
		&res.IsActive,
		&res.ViewCount,
		&res.Photo,
		&res.Video,
		&res.UserID,
		&res.CreatedAt,
		&res.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO news_translations (news_id, language_id, title, description)
		SELECT $1, t.language_id, t.title, t.description
		FROM jsonb_to_recordset(COALESCE(NULLIF($2::jsonb, 'null'::jsonb), '[]'::jsonb))
			AS t(language_id INT, title TEXT, description TEXT)
		ORDER BY t.language_id
		ON CONFLICT (news_id, language_id) DO UPDATE
		SET title = EXCLUDED.title, description = EXCLUDED.description, updated_at = CURRENT_TIMESTAMP
	`, id, translations)

	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(ctx, `
		DELETE FROM news_translations
		WHERE news_id = $1 AND language_id NOT IN (
			SELECT t.language_id
			FROM jsonb_to_recordset(COALESCE(NULLIF($2::jsonb, 'null'::jsonb), '[]'::jsonb)) AS t(language_id INT)
		)
	`, id, translations)

	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(ctx, "DELETE FROM news_categories WHERE news_id = $1", id)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO news_categories (news_id, category_id)
		SELECT $1, category_id FROM unnest($2::INT[]) AS c(category_id)
		ORDER BY category_id
	`, id, req.CategoryIDs)

	if err != nil {
		return nil, err
	}

	err = tx.QueryRow(ctx, `
		SELECT ARRAY(
			SELECT nc.category_id::BIGINT FROM news_categories nc
			WHERE nc.news_id = $1 ORDER BY nc.category_id
		), COALESCE((
			SELECT jsonb_agg(to_jsonb(t) ORDER BY t.language_id)
			FROM news_translations t WHERE t.news_id = $1
		), '[]'::jsonb)
	`, id).Scan(&res.CategoryIDs, &res.Translations)

	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &res, nil
}

func (repository *newsRepository) Delete(ctx context.Context, id int64) error {
	tx, err := repository.db.Begin(ctx)
	{
		if err != nil {
			return err
		}
	}

	defer tx.Rollback(context.WithoutCancel(ctx))

	var newsID int64

	err = tx.QueryRow(ctx, "SELECT id FROM news WHERE id = $1 FOR UPDATE", id).Scan(&newsID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, "DELETE FROM news_categories WHERE news_id = $1", id)
	{
		if err != nil {
			return err
		}
	}

	_, err = tx.Exec(ctx, "DELETE FROM news_translations WHERE news_id = $1", id)
	{
		if err != nil {
			return err
		}
	}

	result, err := tx.Exec(ctx, "DELETE FROM news WHERE id = $1", id)
	{

		if err != nil {
			return err
		}
	}

	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return tx.Commit(ctx)
}
