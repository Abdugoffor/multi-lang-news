package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"test/config"
	"test/helper"
	category_cmd "test/module/category"
	language_cmd "test/module/language"
	news_cmd "test/module/news"

	"github.com/julienschmidt/httprouter"
)

func main() {
	helper.LoadEnv()

	migrate()

	db := config.DBConnect()

	defer db.Close()

	redis := config.RedisConnect()
	defer redis.Close()

	router := httprouter.New()
	{
		language_cmd.Cmd(router, "/api", db)
		category_cmd.Cmd(router, "/api", db)
		news_cmd.Cmd(router, "/api", db)
		// auth_cmd.Register(router, "/api", db)
	}

	fmt.Printf("🚀 Server started on http://localhost:%s\n", helper.ENV("APP_PORT"))

	if err := http.ListenAndServe(":"+helper.ENV("APP_PORT"), router); err != nil {

		log.Fatal(err)
	}
}

func migrate() {
	if len(os.Args) > 1 {

		switch os.Args[1] {

		case "migrate:create":

			if len(os.Args) < 3 {
				fmt.Println("Usage:   go run . migrate:create <name>")

				fmt.Println("Example: go run . migrate:create add_code_to_regions")

				os.Exit(1)
			}

			config.MigrateCreate(os.Args[2])

			os.Exit(0)
		}
	}
}
