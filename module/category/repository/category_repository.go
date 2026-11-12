package category_repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	category_dto "test/module/category/dto"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrLanguageUnavailable = errors.New("translation language is no longer active or available")

type CategoryRepository interface {
	List(ctx context.Context, filter *category_dto.Filter) ([]category_dto.ListItem, error)
	HasMore(ctx context.Context, filter *category_dto.Filter, cursor int64, direction string) (bool, error)
	Show(ctx context.Context, id int64) (*category_dto.Response, error)
	Create(ctx context.Context, req *category_dto.Create) (*category_dto.Response, error)
	Update(ctx context.Context, id int64, req *category_dto.Update) (*category_dto.Response, error)
	Delete(ctx context.Context, id int64) error
}

type categoryRepository struct {
	db *pgxpool.Pool
}

func NewCategoryRepository(db *pgxpool.Pool) CategoryRepository {
	return &categoryRepository{db: db}
}

func (repository *categoryRepository) List(ctx context.Context, filter *category_dto.Filter) ([]category_dto.ListItem, error) {
	condition := "TRUE"

	args := []any{filter.LanguageID}

	order := "DESC"

	if filter.Direction == category_dto.DirectionPrev {
		order = "ASC"
	}

	if filter.Cursor > 0 {
		args = append(args, filter.Cursor)
		condition = "c.id < $2::BIGINT"

		if filter.Direction == category_dto.DirectionPrev {
			condition = "c.id > $2::BIGINT"
		}
	}

	if filter.Name != "" {
		args = append(args, filter.Name)
		condition += fmt.Sprintf(" AND COALESCE(t.name, c.default_name) ILIKE '%%' || $%d || '%%'", len(args))
	}

	if filter.IsActive != nil {
		args = append(args, *filter.IsActive)
		condition += fmt.Sprintf(" AND c.is_active = $%d", len(args))
	}

	args = append(args, filter.Limit+1)

	query := fmt.Sprintf(`
		SELECT c.id, COALESCE(t.name, c.default_name), c.is_active, c.sort_order, c.created_at, c.updated_at
		FROM categories c
		LEFT JOIN category_translations t ON t.category_id = c.id AND t.language_id = $1::BIGINT
		WHERE %s
		ORDER BY c.id %s
		LIMIT $%d
	`, condition, order, len(args))

	rows, err := repository.db.Query(ctx, query, args...)
	{
		if err != nil {
			return nil, err
		}
	}

	defer rows.Close()

	result := make([]category_dto.ListItem, 0, filter.Limit+1)

	for rows.Next() {
		var res category_dto.ListItem

		err := rows.Scan(
			&res.ID,
			&res.Name,
			&res.IsActive,
			&res.SortOrder,
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

func (repository *categoryRepository) HasMore(ctx context.Context, filter *category_dto.Filter, cursor int64, direction string) (bool, error) {
	condition := "c.id < $2::BIGINT"
	args := []any{filter.LanguageID, cursor}

	if direction == category_dto.DirectionPrev {
		condition = "c.id > $2::BIGINT"
	}

	if filter.Name != "" {
		args = append(args, filter.Name)
		condition += fmt.Sprintf(" AND COALESCE(t.name, c.default_name) ILIKE '%%' || $%d || '%%'", len(args))
	}

	if filter.IsActive != nil {
		args = append(args, *filter.IsActive)
		condition += fmt.Sprintf(" AND c.is_active = $%d", len(args))
	}

	query := fmt.Sprintf(`
		SELECT EXISTS (
			SELECT 1 FROM categories c
			LEFT JOIN category_translations t ON t.category_id = c.id AND t.language_id = $1::BIGINT
			WHERE %s
		)
	`, condition)

	var exists bool

	err := repository.db.QueryRow(ctx, query, args...).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func (repository *categoryRepository) Show(ctx context.Context, id int64) (*category_dto.Response, error) {
	var res category_dto.Response

	err := repository.db.QueryRow(ctx, `
		SELECT c.id, c.default_name, c.is_active, c.sort_order,
			COALESCE((
				SELECT jsonb_agg(jsonb_build_object('language_id', t.language_id, 'name', t.name)
					ORDER BY t.language_id)
				FROM category_translations t WHERE t.category_id = c.id
			), '[]'::jsonb),
			c.created_at, c.updated_at
		FROM categories c WHERE c.id = $1
	`, id).Scan(
		&res.ID,
		&res.Default,
		&res.IsActive,
		&res.SortOrder,
		&res.Translations,
		&res.CreatedAt,
		&res.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &res, nil
}

func (repository *categoryRepository) Create(ctx context.Context, req *category_dto.Create) (*category_dto.Response, error) {
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

	var res category_dto.Response

	err = tx.QueryRow(ctx, `
		INSERT INTO categories (default_name, is_active, sort_order)
		VALUES ($1, $2, $3)
		RETURNING id, default_name, is_active, sort_order, created_at, updated_at
	`, req.Default, req.IsActive, req.SortOrder).Scan(
		&res.ID,
		&res.Default,
		&res.IsActive,
		&res.SortOrder,
		&res.CreatedAt,
		&res.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	result, err := tx.Exec(ctx, `
		INSERT INTO category_translations (category_id, language_id, name)
		SELECT $1, l.id, t.name
		FROM jsonb_to_recordset(COALESCE(NULLIF($2::jsonb, 'null'::jsonb), '[]'::jsonb)) AS t(language_id INT, name TEXT)
		JOIN languages l ON l.id = t.language_id
		WHERE l.is_active = TRUE
		ORDER BY l.id
		FOR SHARE OF l
	`, res.ID, translations)
	if err != nil {
		return nil, err
	}

	if result.RowsAffected() != int64(len(req.Translations)) {
		return nil, ErrLanguageUnavailable
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	res.Translations = append(make([]category_dto.Translation, 0, len(req.Translations)), req.Translations...)

	return &res, nil
}

func (repository *categoryRepository) Update(ctx context.Context, id int64, req *category_dto.Update) (*category_dto.Response, error) {
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

	var res category_dto.Response

	err = tx.QueryRow(ctx, `
		UPDATE categories
		SET default_name = $1, is_active = $2, sort_order = $3, updated_at = CURRENT_TIMESTAMP
		WHERE id = $4
		RETURNING id, default_name, is_active, sort_order, created_at, updated_at
	`, req.Default, req.IsActive, req.SortOrder, id).Scan(
		&res.ID,
		&res.Default,
		&res.IsActive,
		&res.SortOrder,
		&res.CreatedAt,
		&res.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	var saved int64

	err = tx.QueryRow(ctx, `
		WITH active_translations AS MATERIALIZED (
			SELECT l.id AS language_id, t.name
			FROM jsonb_to_recordset(COALESCE(NULLIF($2::jsonb, 'null'::jsonb), '[]'::jsonb)) AS t(language_id INT, name TEXT)
			JOIN languages l ON l.id = t.language_id
			WHERE l.is_active = TRUE
			ORDER BY l.id
			FOR SHARE OF l
		), updated AS (
			UPDATE category_translations t
			SET name = a.name, updated_at = CURRENT_TIMESTAMP
			FROM active_translations a
			WHERE t.category_id = $1 AND t.language_id = a.language_id
			RETURNING t.language_id
		), inserted AS (
			INSERT INTO category_translations (category_id, language_id, name)
			SELECT $1, a.language_id, a.name FROM active_translations a
			WHERE NOT EXISTS (SELECT 1 FROM updated u WHERE u.language_id = a.language_id)
			RETURNING language_id
		)
		SELECT (SELECT COUNT(*) FROM updated) + (SELECT COUNT(*) FROM inserted)
	`, id, translations).Scan(&saved)
	if err != nil {
		return nil, err
	}

	if saved != int64(len(req.Translations)) {
		return nil, ErrLanguageUnavailable
	}

	_, err = tx.Exec(ctx, `
		DELETE FROM category_translations
		WHERE category_id = $1 AND language_id NOT IN (
			SELECT t.language_id
			FROM jsonb_to_recordset(COALESCE(NULLIF($2::jsonb, 'null'::jsonb), '[]'::jsonb)) AS t(language_id INT)
		)
	`, id, translations)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	res.Translations = append(make([]category_dto.Translation, 0, len(req.Translations)), req.Translations...)

	return &res, nil
}

func (repository *categoryRepository) Delete(ctx context.Context, id int64) error {
	tx, err := repository.db.Begin(ctx)
	{
		if err != nil {
			return err
		}
	}

	defer tx.Rollback(context.WithoutCancel(ctx))

	// Serialize translation writes with deletion, including concurrent PUTs.
	var categoryID int64

	err = tx.QueryRow(ctx, "SELECT id FROM categories WHERE id = $1 FOR UPDATE", id).Scan(&categoryID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, "DELETE FROM category_translations WHERE category_id = $1", id)
	if err != nil {
		return err
	}

	result, err := tx.Exec(ctx, "DELETE FROM categories WHERE id = $1", id)
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
