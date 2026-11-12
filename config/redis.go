package config

import (
	"context"
	"log"
	"net"
	"strconv"
	"time"

	"test/helper"

	"github.com/redis/go-redis/v9"
)

var Redis *redis.Client

func RedisConnect() *redis.Client {
	host := helper.ENV("REDIS_HOST")
	if host == "" {
		host = "127.0.0.1"
	}

	port := helper.ENV("REDIS_PORT")
	if port == "" {
		port = "6380"
	}

	var database int
	if value := helper.ENV("REDIS_DB"); value != "" {
		parsed, err := strconv.Atoi(value)
		{
			if err != nil || parsed < 0 {
				log.Fatal("REDIS_DB must be a nonnegative integer")
			}
		}

		database = parsed
	}

	Redis = redis.NewClient(&redis.Options{
		Addr:                  net.JoinHostPort(host, port),
		Password:              helper.ENV("REDIS_PASSWORD"),
		DB:                    database,
		DialTimeout:           time.Second,
		ReadTimeout:           time.Second,
		WriteTimeout:          time.Second,
		PoolTimeout:           time.Second,
		MaxRetries:            -1,
		ContextTimeoutEnabled: true,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := Redis.Ping(ctx).Err(); err != nil {
		log.Printf("Redis unavailable; active languages will use PostgreSQL: %v", err)
	} else {
		log.Println("Connected to Redis")
	}

	return Redis
}
