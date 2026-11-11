package lang_handler

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"test/helper"
	lang_dto "test/module/language/dto"
	lang_service "test/module/language/service"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/julienschmidt/httprouter"
)

type langHandler struct {
	service *lang_service.LangService
}

func NewLangHandler(router *httprouter.Router, group string, db *pgxpool.Pool) {
	handler := &langHandler{
		service: lang_service.NewLangService(db),
	}

	routes := group + "/languages"
	{
		router.GET(routes, handler.List)
		router.GET(routes+"-active", handler.ListActive)
		router.GET(routes+"/:id", handler.Show)
		router.POST(routes, handler.Create)
		router.PUT(routes+"/:id", handler.Update)
		router.DELETE(routes+"/:id", handler.Delete)
	}
}

func (handler *langHandler) Create(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var req lang_dto.Create
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

func (handler *langHandler) List(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	query := r.URL.Query()

	filter := lang_dto.Filter{
		Limit:     10,
		Name:      strings.TrimSpace(query.Get("name")),
		Direction: query.Get("direction"),
	}

	if filter.Direction == "" {
		filter.Direction = lang_dto.DirectionNext
	}

	if filter.Direction != lang_dto.DirectionNext && filter.Direction != lang_dto.DirectionPrev {
		helper.JSON(w, http.StatusBadRequest, map[string]string{"error": "direction must be next or prev"})
		return
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

	if filter.Direction == lang_dto.DirectionPrev && filter.Cursor == 0 {
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

func (handler *langHandler) Show(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
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

func (handler *langHandler) ListActive(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	res, err := handler.service.ListActive(r.Context())
	{
		if err != nil {
			writeServiceError(w, err)
			return
		}
	}

	helper.JSON(w, http.StatusOK, res)
}

func (handler *langHandler) Update(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	id, ok := helper.ParseID(w, params)
	{
		if !ok {
			return
		}
	}

	var body lang_dto.Create
	{
		if !parseBody(w, r, &body) {
			return
		}
	}

	req := lang_dto.Update(body)

	res, err := handler.service.Update(r.Context(), id, &req)
	{
		if err != nil {
			writeServiceError(w, err)
			return
		}
	}

	helper.JSON(w, http.StatusOK, res)
}

func (handler *langHandler) Delete(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
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

func parseBody(w http.ResponseWriter, r *http.Request, req *lang_dto.Create) bool {
	{
		if err := helper.DecodeJSON(w, r, req); err != nil {
			helper.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return false
		}
	}

	req.Name = strings.TrimSpace(req.Name)

	if errs := helper.Validate(req); errs != nil {
		helper.JSON(w, http.StatusBadRequest, map[string]any{"errors": errs})
		return false
	}

	return true
}

func writeServiceError(w http.ResponseWriter, err error) {
	{
		if errors.Is(err, pgx.ErrNoRows) {
			helper.JSON(w, http.StatusNotFound, map[string]string{"error": "language not found"})
			return
		}

		if errors.Is(err, lang_service.ErrInvalidLang) || errors.Is(err, lang_service.ErrInvalidFilter) {
			helper.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		if errors.Is(err, lang_service.ErrLanguageConflict) || errors.Is(err, lang_service.ErrConcurrentChange) {
			helper.JSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
	}

	log.Printf("Language service error: %v", err)

	helper.JSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
}
