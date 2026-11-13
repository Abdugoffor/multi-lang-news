package news_service

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"

	"test/config"
	"test/helper"
	category_service "test/module/category/service"
	lang_service "test/module/language/service"
	news_dto "test/module/news/dto"
	news_repository "test/module/news/repository"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

var (
	ErrInvalidNews        = errors.New("invalid news data")
	ErrInvalidTranslation = errors.New("translations must contain only active languages, unique language_id values, nonblank titles of at most 255 characters and nonblank descriptions")
	ErrInvalidCategory    = errors.New("category_ids must contain unique existing category IDs")
	ErrInvalidFilter      = errors.New("invalid news filter")
	ErrNewsConflict       = errors.New("news has related records or conflicting translations; remove or move them first")
	ErrConcurrentChange   = errors.New("news changed concurrently; retry the request")
)

type MissingTranslationsError struct {
	Languages []news_dto.MissingLanguage
}

func (err *MissingTranslationsError) Error() string {
	return "translations are required for all active languages"
}

type NewsService struct {
	repository news_repository.NewsRepository
	cache      *redis.Client
	newsKey    string
	language   *lang_service.LangService
	category   *category_service.CategoryService
}

func NewNewsService(db *pgxpool.Pool) *NewsService {
	return &NewsService{
		repository: news_repository.NewNewsRepository(db),
		cache:      config.Redis,
		newsKey:    "test:" + helper.ENV("DB_NAME") + ":news:views:",
		language:   lang_service.NewLangService(db),
		category:   category_service.NewCategoryService(db),
	}
}

func (service *NewsService) List(ctx context.Context, filter *news_dto.Filter) (*news_dto.ListResponse, error) {
	var prepared news_dto.Filter
	{
		if filter != nil {
			prepared = *filter
		}

		if prepared.Limit < 1 || prepared.Limit > 100 {
			prepared.Limit = 10
		}

		if prepared.Direction == "" {
			prepared.Direction = news_dto.DirectionNext
		}

		if prepared.Cursor < 0 || prepared.LanguageID < 0 || prepared.LanguageID > 2147483647 ||
			prepared.CategoryID < 0 || prepared.CategoryID > 2147483647 || prepared.UserID < 0 ||
			(prepared.Direction != news_dto.DirectionNext && prepared.Direction != news_dto.DirectionPrev) ||
			(prepared.Direction == news_dto.DirectionPrev && prepared.Cursor == 0) {
			return nil, ErrInvalidFilter
		}

		prepared.Title = strings.TrimSpace(prepared.Title)
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

	if prepared.Direction == news_dto.DirectionPrev {
		slices.Reverse(rows)
	}

	res := &news_dto.ListResponse{Data: rows}

	if len(rows) == 0 {
		return res, nil
	}

	firstID := rows[0].ID
	lastID := rows[len(rows)-1].ID

	if prepared.Direction == news_dto.DirectionPrev {
		res.HasPrev = hasMore

		res.HasNext, err = service.repository.HasMore(ctx, &prepared, lastID, news_dto.DirectionNext)
		{
			if err != nil {
				return nil, err
			}
		}

	} else {
		res.HasNext = hasMore

		if prepared.Cursor > 0 {
			res.HasPrev, err = service.repository.HasMore(ctx, &prepared, firstID, news_dto.DirectionPrev)
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

func (service *NewsService) Show(ctx context.Context, id int64) (*news_dto.Response, error) {
	return service.repository.Show(ctx, id)
}

func (service *NewsService) View(ctx context.Context, id int64) (*news_dto.Response, error) {
	res, err := service.repository.Show(ctx, id)
	{
		if err != nil {
			return nil, err
		}
	}

	if service.cache == nil {
		return nil, errors.New("news Redis is unavailable")
	}

	key := service.newsKey + strconv.FormatInt(id, 10)

	// Kalit yo'q bo'lsa, hisobni bazadagi sondan boshlaymiz.
	err = service.cache.SetNX(ctx, key, res.ViewCount, 0).Err()
	{
		if err != nil {
			return nil, err
		}
	}

	// Redisdagi hisobni 1 ga oshiramiz va yangi sonni javobga qo'yamiz.
	res.ViewCount, err = service.cache.Incr(ctx, key).Result()
	{
		if err != nil {
			return nil, err
		}
	}

	return res, nil
}

func (service *NewsService) Create(ctx context.Context, req *news_dto.Create) (*news_dto.Response, error) {
	if err := service.prepare(ctx, req); err != nil {
		return nil, err
	}

	res, err := service.repository.Create(ctx, req)
	{
		if err != nil {
			return nil, serviceError(err)
		}
	}

	return res, nil
}

func (service *NewsService) Update(ctx context.Context, id int64, req *news_dto.Update) (*news_dto.Response, error) {
	if err := service.prepare(ctx, (*news_dto.Create)(req)); err != nil {
		return nil, err
	}

	res, err := service.repository.Update(ctx, id, req)
	{
		if err != nil {
			return nil, serviceError(err)
		}
	}

	return res, nil
}

func (service *NewsService) Delete(ctx context.Context, id int64) error {
	return serviceError(service.repository.Delete(ctx, id))
}

func (service *NewsService) prepare(ctx context.Context, req *news_dto.Create) error {
	if req == nil {
		return ErrInvalidNews
	}

	req.Photo = strings.TrimSpace(req.Photo)
	req.Video = strings.TrimSpace(req.Video)

	translations := make(map[int64]news_dto.Translation, len(req.Translations))

	for i, translation := range req.Translations {
		if _, exists := translations[translation.LanguageID]; exists {
			return ErrInvalidTranslation
		}

		req.Translations[i].Title = strings.TrimSpace(translation.Title)
		req.Translations[i].Description = strings.TrimSpace(translation.Description)
		translations[translation.LanguageID] = req.Translations[i]
	}

	languages, err := service.language.ListActive(ctx)
	{
		if err != nil {
			return err
		}
	}

	missingLanguages := make([]news_dto.MissingLanguage, 0)

	for _, language := range languages {
		translation, exists := translations[language.ID]

		if !exists || translation.Title == "" || translation.Description == "" {
			missingLanguages = append(missingLanguages, news_dto.MissingLanguage{
				LanguageID: language.ID,
				Name:       language.Name,
			})
		}
	}

	if len(missingLanguages) > 0 {
		return &MissingTranslationsError{Languages: missingLanguages}
	}

	if len(translations) != len(languages) {
		return ErrInvalidTranslation
	}

	if errs := helper.Validate(req); errs != nil {
		return ErrInvalidNews
	}

	categories := make(map[int64]struct{}, len(req.CategoryIDs))

	for _, categoryID := range req.CategoryIDs {
		if _, exists := categories[categoryID]; exists {
			return ErrInvalidCategory
		}

		categories[categoryID] = struct{}{}

		_, err := service.category.Show(ctx, categoryID)
		{
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrInvalidCategory
			}

			if err != nil {
				return err
			}
		}
	}

	return nil
}

func serviceError(err error) error {
	var pgErr *pgconn.PgError

	if errors.As(err, &pgErr) {
		if pgErr.Code == "40001" || pgErr.Code == "40P01" {
			return ErrConcurrentChange
		}

		if pgErr.Code == "23503" || pgErr.Code == "23505" {
			return ErrNewsConflict
		}
	}

	return err
}
