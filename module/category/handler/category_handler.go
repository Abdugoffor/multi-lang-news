package category_handler

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"test/helper"
	category_dto "test/module/category/dto"
	category_service "test/module/category/service"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/julienschmidt/httprouter"
)

type categoryHandler struct {
	service *category_service.CategoryService
}

func NewCategoryHandler(router *httprouter.Router, group string, db *pgxpool.Pool) {
	handler := &categoryHandler{
		service: category_service.NewCategoryService(db),
	}

	routes := group + "/categories"
	{
		router.GET(routes, handler.List)
		router.GET(routes+"/:id", handler.Show)
		router.POST(routes, handler.Create)
		router.PUT(routes+"/:id", handler.Update)
		router.DELETE(routes+"/:id", handler.Delete)
	}
}

func (handler *categoryHandler) List(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	query := r.URL.Query()

	filter := category_dto.Filter{
		Limit:     10,
		Name:      strings.TrimSpace(query.Get("name")),
		Direction: query.Get("direction"),
	}

	if filter.Direction == "" {
		filter.Direction = category_dto.DirectionNext
	}

	if filter.Direction != category_dto.DirectionNext && filter.Direction != category_dto.DirectionPrev {
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

	if filter.Direction == category_dto.DirectionPrev && filter.Cursor == 0 {
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

	res, err := handler.service.List(r.Context(), &filter)
	{
		if err != nil {
			writeServiceError(w, err)
			return
		}
	}

	helper.JSON(w, http.StatusOK, res)
}

func (handler *categoryHandler) Show(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
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

func (handler *categoryHandler) Create(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var req category_dto.Create
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

func (handler *categoryHandler) Update(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	id, ok := helper.ParseID(w, params)
	{
		if !ok {
			return
		}
	}

	var body category_dto.Create
	{
		if !parseBody(w, r, &body) {
			return
		}
	}

	req := category_dto.Update(body)

	res, err := handler.service.Update(r.Context(), id, &req)
	{
		if err != nil {
			writeServiceError(w, err)
			return
		}
	}

	helper.JSON(w, http.StatusOK, res)
}

func (handler *categoryHandler) Delete(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	id, ok := helper.ParseID(w, params)
	{
		if !ok {
			return
		}

		if err := handler.service.Delete(r.Context(), id); err != nil {
			writeServiceError(w, err)
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseBody(w http.ResponseWriter, r *http.Request, req *category_dto.Create) bool {
	{
		if err := helper.DecodeJSON(w, r, req); err != nil {
			helper.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return false
		}
	}

	req.Default = strings.TrimSpace(req.Default)

	if errs := helper.Validate(req); errs != nil {
		helper.JSON(w, http.StatusBadRequest, map[string]any{"errors": errs})
		return false
	}

	return true
}

func writeServiceError(w http.ResponseWriter, err error) {
	var missingTranslations *category_service.MissingTranslationsError
	{
		if errors.As(err, &missingTranslations) {
			helper.JSON(w, http.StatusBadRequest, category_dto.MissingTranslationsResponse{
				Error:            missingTranslations.Error(),
				MissingLanguages: missingTranslations.Languages,
			})
			return
		}
	}

	{
		if errors.Is(err, pgx.ErrNoRows) {
			helper.JSON(w, http.StatusNotFound, map[string]string{"error": "category not found"})
			return
		}

		if errors.Is(err, category_service.ErrInvalidCategory) || errors.Is(err, category_service.ErrInvalidTranslation) || errors.Is(err, category_service.ErrInvalidFilter) {
			helper.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		if errors.Is(err, category_service.ErrCategoryConflict) || errors.Is(err, category_service.ErrConcurrentChange) {
			helper.JSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
	}

	log.Printf("Category service error: %v", err)

	helper.JSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
}
