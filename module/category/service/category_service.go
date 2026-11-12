package category_service

import (
	"context"
	"errors"
	"slices"
	"strings"

	"test/helper"
	category_dto "test/module/category/dto"
	category_repository "test/module/category/repository"
	lang_service "test/module/language/service"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidCategory    = errors.New("default must be nonblank and at most 255 characters; sort_order must be a nonnegative integer")
	ErrInvalidTranslation = errors.New("translations are required for every active language, with unique language_id values and nonblank names of at most 255 characters")
	ErrInvalidFilter      = errors.New("invalid category filter")
	ErrCategoryConflict   = errors.New("category has related records or conflicting translations; remove or move them first")
	ErrConcurrentChange   = errors.New("category changed concurrently; retry the request")
)

type MissingTranslationsError struct {
	Languages []category_dto.MissingLanguage
}

func (err *MissingTranslationsError) Error() string {
	return "translations are required for all active languages"
}

type CategoryService struct {
	repository category_repository.CategoryRepository
	language   *lang_service.LangService
}

func NewCategoryService(db *pgxpool.Pool) *CategoryService {
	return &CategoryService{
		repository: category_repository.NewCategoryRepository(db),
		language:   lang_service.NewLangService(db),
	}
}

func (service *CategoryService) List(ctx context.Context, filter *category_dto.Filter) (*category_dto.ListResponse, error) {
	var prepared category_dto.Filter
	{
		if filter != nil {
			prepared = *filter
		}

		if prepared.Limit < 1 || prepared.Limit > 100 {
			prepared.Limit = 10
		}

		if prepared.Direction == "" {
			prepared.Direction = category_dto.DirectionNext
		}

		if prepared.Cursor < 0 || prepared.LanguageID < 0 || prepared.LanguageID > 2147483647 ||
			(prepared.Direction != category_dto.DirectionNext && prepared.Direction != category_dto.DirectionPrev) ||
			(prepared.Direction == category_dto.DirectionPrev && prepared.Cursor == 0) {
			return nil, ErrInvalidFilter
		}

		prepared.Name = strings.TrimSpace(prepared.Name)
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

	if prepared.Direction == category_dto.DirectionPrev {
		slices.Reverse(rows)
	}

	res := &category_dto.ListResponse{Data: rows}

	if len(rows) == 0 {
		return res, nil
	}

	firstID := rows[0].ID
	lastID := rows[len(rows)-1].ID

	if prepared.Direction == category_dto.DirectionPrev {
		res.HasPrev = hasMore

		res.HasNext, err = service.repository.HasMore(ctx, &prepared, lastID, category_dto.DirectionNext)
		{
			if err != nil {
				return nil, err
			}
		}

	} else {
		res.HasNext = hasMore

		if prepared.Cursor > 0 {
			res.HasPrev, err = service.repository.HasMore(ctx, &prepared, firstID, category_dto.DirectionPrev)
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

func (service *CategoryService) Show(ctx context.Context, id int64) (*category_dto.Response, error) {
	return service.repository.Show(ctx, id)
}

func (service *CategoryService) Create(ctx context.Context, req *category_dto.Create) (*category_dto.Response, error) {
	if req == nil {
		return nil, ErrInvalidCategory
	}

	req.Default = strings.TrimSpace(req.Default)

	translations := make(map[int64]string, len(req.Translations))

	for i, translation := range req.Translations {
		if _, exists := translations[translation.LanguageID]; exists {
			return nil, ErrInvalidTranslation
		}

		req.Translations[i].Name = strings.TrimSpace(translation.Name)
		translations[translation.LanguageID] = req.Translations[i].Name
	}

	languages, err := service.language.ListActive(ctx)
	{
		if err != nil {
			return nil, err
		}
	}

	missingLanguages := make([]category_dto.MissingLanguage, 0)

	for _, language := range languages {
		name, exists := translations[language.ID]
		if !exists || name == "" {
			missingLanguages = append(missingLanguages, category_dto.MissingLanguage{
				LanguageID: language.ID,
				Name:       language.Name,
			})
		}
	}

	if len(missingLanguages) > 0 {
		return nil, &MissingTranslationsError{Languages: missingLanguages}
	}

	if len(translations) != len(languages) {
		return nil, ErrInvalidTranslation
	}

	if errs := helper.Validate(req); errs != nil {
		return nil, ErrInvalidCategory
	}

	res, err := service.repository.Create(ctx, req)
	{
		if err != nil {
			return nil, serviceError(err)
		}
	}

	return res, nil
}

func (service *CategoryService) Update(ctx context.Context, id int64, req *category_dto.Update) (*category_dto.Response, error) {
	if req == nil {
		return nil, ErrInvalidCategory
	}

	req.Default = strings.TrimSpace(req.Default)

	translations := make(map[int64]string, len(req.Translations))

	for i, translation := range req.Translations {
		if _, exists := translations[translation.LanguageID]; exists {
			return nil, ErrInvalidTranslation
		}

		req.Translations[i].Name = strings.TrimSpace(translation.Name)
		translations[translation.LanguageID] = req.Translations[i].Name
	}

	languages, err := service.language.ListActive(ctx)
	{
		if err != nil {
			return nil, err
		}
	}

	missingLanguages := make([]category_dto.MissingLanguage, 0)

	for _, language := range languages {

		name, exists := translations[language.ID]

		if !exists || name == "" {
			missingLanguages = append(missingLanguages, category_dto.MissingLanguage{
				LanguageID: language.ID,
				Name:       language.Name,
			})
		}
	}

	if len(missingLanguages) > 0 {
		return nil, &MissingTranslationsError{Languages: missingLanguages}
	}

	if len(translations) != len(languages) {
		return nil, ErrInvalidTranslation
	}

	if errs := helper.Validate(req); errs != nil {
		return nil, ErrInvalidCategory
	}

	res, err := service.repository.Update(ctx, id, req)
	{
		if err != nil {
			return nil, serviceError(err)
		}
	}

	return res, nil
}

func (service *CategoryService) Delete(ctx context.Context, id int64) error {
	return serviceError(service.repository.Delete(ctx, id))
}

func serviceError(err error) error {
	var pgErr *pgconn.PgError

	if errors.As(err, &pgErr) {
		if pgErr.Code == "40001" || pgErr.Code == "40P01" {
			return ErrConcurrentChange
		}

		if pgErr.Code == "23503" || pgErr.Code == "23505" {
			return ErrCategoryConflict
		}
	}

	return err
}
