package language_cmd

import (
	lang_handler "test/module/language/handler"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/julienschmidt/httprouter"
)

func Cmd(router *httprouter.Router, group string, db *pgxpool.Pool) {
	lang_handler.NewLangHandler(router, group, db)
}
