package config

import (
	"context"
	"fmt"
	"log"
	"test/helper"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var DB *pgxpool.Pool

func DBConnect() *pgxpool.Pool {
	driver := helper.ENV("DB_DRIVER")

	if driver != "postgres" {
		log.Fatal("❌ pgx faqat PostgreSQL bilan ishlaydi (DB_DRIVER=postgres)")
	}

	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s&timezone=%s",
		helper.ENV("DB_USER"),
		helper.ENV("DB_PASSWORD"),
		helper.ENV("DB_HOST"),
		helper.ENV("DB_PORT"),
		helper.ENV("DB_NAME"),
		helper.ENV("DB_SSLMODE"),
		helper.ENV("DB_TIMEZONE"),
	)

	ctx := context.Background()

	config, err := pgxpool.ParseConfig(dsn)
	{
		if err != nil {
			log.Fatal("❌ DSN parse error:", err)
		}
	}

	config.MaxConns = 50
	config.MinConns = 10
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 30 * time.Minute
	config.HealthCheckPeriod = time.Minute

	db, err := pgxpool.NewWithConfig(ctx, config)
	{
		if err != nil {
			log.Fatal("❌ Failed to connect to PostgreSQL:", err)
		}
	}

	// Ping
	if err := db.Ping(ctx); err != nil {
		log.Fatal("❌ DB ping error:", err)
	}

	log.Println("✅ Connected to PostgreSQL (pgxpool) 🚀")

	DB = db

	RunMigrations()

	return db
}
