package lang_service

import (
	"context"
	"errors"
	"log"
	"slices"
	"time"

	lang_dto "test/module/language/dto"
	lang_repository "test/module/language/repository"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidLang      = errors.New("invalid language data")
	ErrInvalidFilter    = errors.New("invalid language filter")
	ErrLanguageConflict = errors.New("language has related records; remove or move them first")
	ErrConcurrentChange = errors.New("language changed concurrently; retry the request")
)

type LangService struct {
	repository lang_repository.LangRepository
}

func NewLangService(db *pgxpool.Pool) *LangService {
	return &LangService{
		repository: lang_repository.NewLangRepository(db),
	}
}

func (service *LangService) Create(ctx context.Context, req *lang_dto.Create) (*lang_dto.Response, error) {

	if req == nil {
		return nil, ErrInvalidLang
	}

	res, err := service.repository.Create(ctx, req)
	{
		if err != nil {
			return nil, serviceError(err)
		}
	}

	service.refreshActive(ctx)

	return res, nil
}

func (service *LangService) List(ctx context.Context, filter *lang_dto.Filter) (*lang_dto.ListResponse, error) {
	var prepared lang_dto.Filter
	{
		if filter != nil {
			prepared = *filter
		}

		if prepared.Limit < 1 || prepared.Limit > 100 {
			prepared.Limit = 10
		}

		if prepared.Direction == "" {
			prepared.Direction = lang_dto.DirectionNext
		}

		if prepared.Cursor < 0 || (prepared.Direction != lang_dto.DirectionNext && prepared.Direction != lang_dto.DirectionPrev) {
			return nil, ErrInvalidFilter
		}
	}

	rows, err := service.repository.List(ctx, &prepared)
	{
		if err != nil {
			return nil, err
		}
	}

	hasMore := len(rows) > prepared.Limit

	if hasMore {
		rows = rows[:prepared.Limit]
	}

	if prepared.Direction == lang_dto.DirectionPrev {
		slices.Reverse(rows)
	}

	res := &lang_dto.ListResponse{Data: rows}

	if len(rows) == 0 {
		return res, nil
	}

	firstID := rows[0].ID

	lastID := rows[len(rows)-1].ID

	if prepared.Direction == lang_dto.DirectionPrev {
		res.HasPrev = hasMore

		res.HasNext, err = service.repository.HasMore(ctx, &prepared, lastID, lang_dto.DirectionNext)
		{
			if err != nil {
				return nil, err
			}
		}

	} else {
		res.HasNext = hasMore

		if prepared.Cursor > 0 {
			res.HasPrev, err = service.repository.HasMore(ctx, &prepared, firstID, lang_dto.DirectionPrev)
			{
				if err != nil {
					return nil, err
				}
			}
		}
	}

	if res.HasNext {
		res.NextCursor = &lastID
	}

	if res.HasPrev {
		res.PrevCursor = &firstID
	}

	return res, nil
}

func (service *LangService) ListActive(ctx context.Context) ([]*lang_dto.Response, error) {
	return service.repository.ListActive(ctx)
}

func (service *LangService) Show(ctx context.Context, id int64) (*lang_dto.Response, error) {
	return service.repository.Show(ctx, id)
}

func (service *LangService) Update(ctx context.Context, id int64, req *lang_dto.Update) (*lang_dto.Response, error) {

	if req == nil {
		return nil, ErrInvalidLang
	}

	_, err := service.repository.Show(ctx, id)
	{
		if err != nil {
			return nil, serviceError(err)
		}
	}

	res, err := service.repository.Update(ctx, id, req)
	{
		if err != nil {
			return nil, serviceError(err)
		}
	}

	service.refreshActive(ctx)

	return res, nil
}

func (service *LangService) Delete(ctx context.Context, id int64) error {
	err := service.repository.Delete(ctx, id)
	{
		if err != nil {
			return serviceError(err)
		}
	}

	service.refreshActive(ctx)

	return nil
}

func (service *LangService) refreshActive(ctx context.Context) {
	// PostgreSQL has already committed; finish cache maintenance after request cancellation.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()

	invalidated, err := service.repository.InvalidateActive(ctx)
	{
		if err != nil {
			log.Printf("Invalidate active languages cache: %v", err)
			return
		}
	}

	if !invalidated {
		return
	}

	_, err = service.ListActive(ctx)
	{
		if err != nil {
			log.Printf("Reload active languages cache: %v", err)
			return
		}
	}
}

func serviceError(err error) error {
	var pgErr *pgconn.PgError

	if errors.As(err, &pgErr) {
		if pgErr.Code == "40001" || pgErr.Code == "40P01" {
			return ErrConcurrentChange
		}

		if pgErr.Code == "23503" {
			return ErrLanguageConflict
		}
	}

	return err
}
