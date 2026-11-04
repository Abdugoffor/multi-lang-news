package config

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func RunMigrations() {
	ctx := context.Background()

	_, err := DB.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			id         SERIAL       PRIMARY KEY,
			name       VARCHAR(255) NOT NULL UNIQUE,
			applied_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
		)
	`)

	if err != nil {
		log.Fatal("❌ schema_migrations table error:", err)
	}

	entries, err := fs.ReadDir(migrationFiles, "migrations")
	{
		if err != nil {
			log.Fatal("❌ migrations dir read error:", err)
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	applied := 0

	for _, entry := range entries {

		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		name := entry.Name()

		var count int
		{
			if err := DB.QueryRow(ctx,
				`SELECT COUNT(*) FROM schema_migrations WHERE name = $1`, name,
			).Scan(&count); err != nil {
				log.Fatalf("❌ Migration check error [%s]: %v", name, err)
			}
		}

		if count > 0 {
			continue
		}

		content, err := migrationFiles.ReadFile("migrations/" + name)
		{
			if err != nil {
				log.Fatalf("❌ Migration read error [%s]: %v", name, err)
			}
		}

		if _, err := DB.Exec(ctx, string(content)); err != nil {
			log.Fatalf("❌ Migration failed [%s]: %v", name, err)
		}

		if _, err := DB.Exec(ctx,
			`INSERT INTO schema_migrations (name) VALUES ($1)`, name,
		); err != nil {
			log.Fatalf("❌ Migration record error [%s]: %v", name, err)
		}

		log.Printf("✅ Migration applied: %s", name)

		applied++
	}

	if applied == 0 {
		log.Println("✅ Migrations: nothing new")
	} else {
		log.Printf("✅ Migrations: %d applied", applied)
	}
}

// make migrate name=add_code_to_regions
// # ✅ Migration created: config/migrations/002_add_code_to_regions.sql

const migrationsDir = "config/migrations"

func MigrateCreate(name string) {

	name = strings.ToLower(strings.TrimSpace(name))

	name = regexp.MustCompile(`[^a-z0-9_]+`).ReplaceAllString(name, "_")

	if name == "" {
		fmt.Println("❌ Migration name cannot be empty")
		os.Exit(1)
	}

	next := nextMigrationNumber()

	fullPath := filepath.Join(migrationsDir, fmt.Sprintf("%03d_%s.sql", next, name))

	if err := os.WriteFile(fullPath, []byte(""), 0644); err != nil {
		fmt.Printf("❌ Failed to create file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Migration created: %s\n", fullPath)
}

func nextMigrationNumber() int {
	entries, err := os.ReadDir(migrationsDir)
	{
		if err != nil {
			fmt.Printf("❌ Cannot read migrations dir: %v\n", err)
			os.Exit(1)
		}
	}

	re := regexp.MustCompile(`^(\d+)_`)
	nums := []int{}

	for _, e := range entries {

		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}

		if m := re.FindStringSubmatch(e.Name()); len(m) > 1 {
			if n, err := strconv.Atoi(m[1]); err == nil {
				nums = append(nums, n)
			}
		}
	}

	if len(nums) > 0 {
		sort.Ints(nums)
		return nums[len(nums)-1] + 1
	}

	return 1
}
