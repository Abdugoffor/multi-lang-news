package lang_repository

import (
	"context"
	"fmt"

	lang_dto "test/module/language/dto"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LangRepository interface {
	List(ctx context.Context, filter *lang_dto.Filter) ([]lang_dto.Response, error)
	HasMore(ctx context.Context, filter *lang_dto.Filter, cursor int64, direction string) (bool, error)
	ListActive(ctx context.Context) ([]*lang_dto.Response, error)
	Show(ctx context.Context, id int64) (*lang_dto.Response, error)
	Create(ctx context.Context, req *lang_dto.Create) (*lang_dto.Response, error)
	Update(ctx context.Context, id int64, req *lang_dto.Update) (*lang_dto.Response, error)
	Delete(ctx context.Context, id int64) error
}

type langRepository struct {
	db *pgxpool.Pool
}

func NewLangRepository(db *pgxpool.Pool) LangRepository {
	return &langRepository{db: db}
}

func (repository *langRepository) List(ctx context.Context, filter *lang_dto.Filter) ([]lang_dto.Response, error) {
	condition := "TRUE"

	args := make([]any, 0, 4)

	order := "DESC"

	if filter.Direction == lang_dto.DirectionPrev {
		order = "ASC"
	}

	if filter.Cursor > 0 {
		args = append(args, filter.Cursor)
		condition = "id < $1::BIGINT"

		if filter.Direction == lang_dto.DirectionPrev {
			condition = "id > $1::BIGINT"
		}
	}

	if filter.Name != "" {
		args = append(args, filter.Name)
		condition += fmt.Sprintf(" AND name ILIKE '%%' || $%d || '%%'", len(args))
	}

	if filter.IsActive != nil {
		args = append(args, *filter.IsActive)
		condition += fmt.Sprintf(" AND is_active = $%d", len(args))
	}

	args = append(args, filter.Limit+1)

	query := fmt.Sprintf(`
		SELECT id, name, is_active, created_at, updated_at
		FROM languages
		WHERE %s
		ORDER BY id %s
		LIMIT $%d
	`, condition, order, len(args))

	rows, err := repository.db.Query(ctx, query, args...)
	{
		if err != nil {
			return nil, err
		}
	}

	defer rows.Close()

	result := make([]lang_dto.Response, 0, filter.Limit+1)

	for rows.Next() {
		var res lang_dto.Response

		err := rows.Scan(
			&res.ID,
			&res.Name,
			&res.IsActive,
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

func (repository *langRepository) HasMore(ctx context.Context, filter *lang_dto.Filter, cursor int64, direction string) (bool, error) {
	condition := "id < $1::BIGINT"

	args := make([]any, 0, 3)

	args = append(args, cursor)

	if direction == lang_dto.DirectionPrev {
		condition = "id > $1::BIGINT"
	}

	if filter.Name != "" {
		args = append(args, filter.Name)
		condition += fmt.Sprintf(" AND name ILIKE '%%' || $%d || '%%'", len(args))
	}

	if filter.IsActive != nil {
		args = append(args, *filter.IsActive)
		condition += fmt.Sprintf(" AND is_active = $%d", len(args))
	}

	query := fmt.Sprintf(`
		SELECT EXISTS (
			SELECT 1 FROM languages
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

func (repository *langRepository) ListActive(ctx context.Context) ([]*lang_dto.Response, error) {
	rows, err := repository.db.Query(ctx, `
		SELECT id, name, is_active, created_at, updated_at
		FROM languages
		WHERE is_active = TRUE
		ORDER BY id DESC
	`)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	result := make([]*lang_dto.Response, 0)

	for rows.Next() {
		var res lang_dto.Response

		err := rows.Scan(
			&res.ID,
			&res.Name,
			&res.IsActive,
			&res.CreatedAt,
			&res.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		result = append(result, &res)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func (repository *langRepository) Show(ctx context.Context, id int64) (*lang_dto.Response, error) {
	var res lang_dto.Response

	err := repository.db.QueryRow(ctx, `
		SELECT id, name, is_active, created_at, updated_at
		FROM languages WHERE id = $1
	`, id).Scan(
		&res.ID,
		&res.Name,
		&res.IsActive,
		&res.CreatedAt,
		&res.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &res, nil
}

func (repository *langRepository) Create(ctx context.Context, req *lang_dto.Create) (*lang_dto.Response, error) {
	var res lang_dto.Response

	err := repository.db.QueryRow(ctx, `
		INSERT INTO languages(name, is_active)
		VALUES ($1, $2)
		RETURNING id, name, is_active, created_at, updated_at
	`, req.Name, req.IsActive).Scan(
		&res.ID,
		&res.Name,
		&res.IsActive,
		&res.CreatedAt,
		&res.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &res, nil
}

func (repository *langRepository) Update(ctx context.Context, id int64, req *lang_dto.Update) (*lang_dto.Response, error) {
	var res lang_dto.Response

	err := repository.db.QueryRow(ctx, `
		UPDATE languages
		SET name = $1, is_active = $2, updated_at = CURRENT_TIMESTAMP
		WHERE id = $3
		RETURNING id, name, is_active, created_at, updated_at
	`, req.Name, req.IsActive, id).Scan(
		&res.ID,
		&res.Name,
		&res.IsActive,
		&res.CreatedAt,
		&res.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &res, nil
}

func (repository *langRepository) Delete(ctx context.Context, id int64) error {
	result, err := repository.db.Exec(ctx, "DELETE FROM languages WHERE id = $1", id)
	{
		if err != nil {
			return err
		}
	}

	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}
