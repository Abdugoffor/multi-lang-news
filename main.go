package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"test/config"
	"test/helper"
	category_cmd "test/module/category"
	language_cmd "test/module/language"
	news_cmd "test/module/news"
	news_service "test/module/news/service"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/julienschmidt/httprouter"
	"github.com/robfig/cron/v3"
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

	scheduler := command(db)

	defer scheduler.Stop()

	fmt.Printf("🚀 Server started on http://localhost:%s\n", helper.ENV("APP_PORT"))

	if err := http.ListenAndServe(":"+helper.ENV("APP_PORT"), router); err != nil {

		log.Fatal(err)
	}
}

func command(db *pgxpool.Pool) *cron.Cron {
	scheduler := cron.New(cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger)))
	service := news_service.NewNewsService(db)

	_, err := scheduler.AddFunc("@every 3m", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		err := service.SyncViews(ctx)
		{
			if err != nil {
				log.Println("Sync news views error:", err)
				return
			}
		}
	})
	{
		if err != nil {
			log.Fatal("Cron schedule error: ", err)
		}
	}

	scheduler.Start()

	return scheduler

	// 10 soniyada bir marta sendEmail funksiyasini ishga tushiradi
	// _, err := scheduler.AddFunc("@every 10s", sendEmail)
	// {
	// 	if err != nil {
	// 		log.Fatal("Cron schedule error: ", err)
	// 	}
	// }

	// _, err = scheduler.AddFunc("@every 20s", func() {
	// 	fmt.Println("20 soniyada bir marta ishga tushadi")
	// })
	// {
	// 	if err != nil {
	// 		log.Fatal("Cron schedule error: ", err)
	// 	}
	// }

}

func sendEmail() {
	fmt.Println("Email ga habar yuborildi")
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
