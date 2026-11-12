package category_cmd

import (
	category_handler "test/module/category/handler"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/julienschmidt/httprouter"
)

func Cmd(router *httprouter.Router, group string, db *pgxpool.Pool) {
	category_handler.NewCategoryHandler(router, group, db)
}
