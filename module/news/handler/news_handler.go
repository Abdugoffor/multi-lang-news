package news_handler

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"test/helper"
	news_dto "test/module/news/dto"
	news_service "test/module/news/service"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/julienschmidt/httprouter"
)

type newsHandler struct {
	service *news_service.NewsService
}

func NewNewsHandler(router *httprouter.Router, group string, db *pgxpool.Pool) {
	handler := &newsHandler{
		service: news_service.NewNewsService(db),
	}

	routes := group + "/news"
	{
		router.GET(routes, handler.List)
		router.GET(routes+"/:id", handler.Show)
		router.GET(routes+"/:id/view", handler.View)
		router.POST(routes, handler.Create)
		router.PUT(routes+"/:id", handler.Update)
		router.DELETE(routes+"/:id", handler.Delete)
	}
}

func (handler *newsHandler) List(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	query := r.URL.Query()

	filter := news_dto.Filter{
		Limit:     10,
		Title:     strings.TrimSpace(query.Get("title")),
		Direction: query.Get("direction"),
	}

	if filter.Direction == "" {
		filter.Direction = news_dto.DirectionNext
	}

	if filter.Direction != news_dto.DirectionNext && filter.Direction != news_dto.DirectionPrev {
		helper.JSON(w, http.StatusBadRequest, map[string]string{"error": "direction must be next or prev"})
		return
	}

	if query.Has("language_id") {
		languageID, err := strconv.ParseInt(query.Get("language_id"), 10, 32)
		{
			if err != nil || languageID < 1 {
				helper.JSON(w, http.StatusBadRequest, map[string]string{"error": "language_id must be a positive integer"})
				return
			}
		}

		filter.LanguageID = languageID
	}

	if query.Has("category_id") {
		categoryID, err := strconv.ParseInt(query.Get("category_id"), 10, 32)
		{
			if err != nil || categoryID < 1 {
				helper.JSON(w, http.StatusBadRequest, map[string]string{"error": "category_id must be a positive integer"})
				return
			}
		}

		filter.CategoryID = categoryID
	}

	if query.Has("user_id") {
		userID, err := strconv.ParseInt(query.Get("user_id"), 10, 64)
		{
			if err != nil || userID < 1 {
				helper.JSON(w, http.StatusBadRequest, map[string]string{"error": "user_id must be a positive integer"})
				return
			}
		}

		filter.UserID = userID
	}

	if query.Has("cursor") {
		cursor, err := strconv.ParseInt(query.Get("cursor"), 10, 64)
		{
			if err != nil || cursor < 1 {
				helper.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid cursor"})
				return
			}
		}

		filter.Cursor = cursor
	}

	if filter.Direction == news_dto.DirectionPrev && filter.Cursor == 0 {
		helper.JSON(w, http.StatusBadRequest, map[string]string{"error": "cursor is required for prev direction"})
		return
	}

	if query.Has("limit") {
		limit, err := strconv.Atoi(query.Get("limit"))
		{
			if err != nil || limit < 1 || limit > 100 {
				helper.JSON(w, http.StatusBadRequest, map[string]string{"error": "limit must be between 1 and 100"})
				return
			}
		}

		filter.Limit = limit
	}

	if query.Has("is_active") {
		raw := query.Get("is_active")

		if raw != "true" && raw != "false" {
			helper.JSON(w, http.StatusBadRequest, map[string]string{"error": "is_active must be true or false"})
			return
		}

		isActive := raw == "true"

		filter.IsActive = &isActive
	}

	if query.Has("is_main") {
		raw := query.Get("is_main")

		if raw != "true" && raw != "false" {
			helper.JSON(w, http.StatusBadRequest, map[string]string{"error": "is_main must be true or false"})
			return
		}

		isMain := raw == "true"

		filter.IsMain = &isMain
	}

	res, err := handler.service.List(r.Context(), &filter)
	{
		if err != nil {
			writeServiceError(w, err)
			return
		}
	}

	helper.JSON(w, http.StatusOK, res)
}

func (handler *newsHandler) Show(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	id, ok := helper.ParseID(w, params)
	{
		if !ok {
			return
		}
	}

	res, err := handler.service.Show(r.Context(), id)
	{
		if err != nil {
			writeServiceError(w, err)
			return
		}
	}

	helper.JSON(w, http.StatusOK, res)
}

func (handler *newsHandler) View(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	w.Header().Set("Cache-Control", "no-store")

	id, ok := helper.ParseID(w, params)
	{
		if !ok {
			return
		}
	}

	res, err := handler.service.View(r.Context(), id)
	{
		if err != nil {
			writeServiceError(w, err)
			return
		}
	}

	helper.JSON(w, http.StatusOK, res)
}

func (handler *newsHandler) Create(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var req news_dto.Create
	{
		if !parseBody(w, r, &req) {
			return
		}
	}

	res, err := handler.service.Create(r.Context(), &req)
	{
		if err != nil {
			writeServiceError(w, err)
			return
		}
	}

	helper.JSON(w, http.StatusCreated, res)
}

func (handler *newsHandler) Update(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	id, ok := helper.ParseID(w, params)
	{
		if !ok {
			return
		}
	}

	var body news_dto.Create
	{
		if !parseBody(w, r, &body) {
			return
		}
	}

	req := news_dto.Update(body)

	res, err := handler.service.Update(r.Context(), id, &req)
	{
		if err != nil {
			writeServiceError(w, err)
			return
		}
	}

	helper.JSON(w, http.StatusOK, res)
}

func (handler *newsHandler) Delete(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	id, ok := helper.ParseID(w, params)
	{
		if !ok {
			return
		}
	}

	err := handler.service.Delete(r.Context(), id)
	{
		if err != nil {
			writeServiceError(w, err)
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseBody(w http.ResponseWriter, r *http.Request, req *news_dto.Create) bool {
	{
		if err := helper.DecodeJSON(w, r, req); err != nil {
			helper.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return false
		}
	}

	req.Photo = strings.TrimSpace(req.Photo)
	req.Video = strings.TrimSpace(req.Video)

	if errs := helper.Validate(req); errs != nil {
		helper.JSON(w, http.StatusBadRequest, map[string]any{"errors": errs})
		return false
	}

	return true
}

func writeServiceError(w http.ResponseWriter, err error) {
	var missingTranslations *news_service.MissingTranslationsError
	{
		if errors.As(err, &missingTranslations) {
			helper.JSON(w, http.StatusBadRequest, news_dto.MissingTranslationsResponse{
				Error:            missingTranslations.Error(),
				MissingLanguages: missingTranslations.Languages,
			})
			return
		}
	}

	{
		if errors.Is(err, pgx.ErrNoRows) {
			helper.JSON(w, http.StatusNotFound, map[string]string{"error": "news not found"})
			return
		}

		if errors.Is(err, news_service.ErrInvalidNews) || errors.Is(err, news_service.ErrInvalidCategory) || errors.Is(err, news_service.ErrInvalidTranslation) || errors.Is(err, news_service.ErrInvalidFilter) {
			helper.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		if errors.Is(err, news_service.ErrNewsConflict) || errors.Is(err, news_service.ErrConcurrentChange) {
			helper.JSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
	}

	log.Printf("News service error: %v", err)

	helper.JSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
}
