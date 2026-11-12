package news_cmd

import (
	news_handler "test/module/news/handler"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/julienschmidt/httprouter"
)

func Cmd(router *httprouter.Router, group string, db *pgxpool.Pool) {
	news_handler.NewNewsHandler(router, group, db)
}
