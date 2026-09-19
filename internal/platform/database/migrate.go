package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"review-api/migrations"
)

func MigrateUp(ctx context.Context, pool *pgxpool.Pool) error {
	db := stdlib.OpenDBFromPool(pool)
	defer func() { _ = db.Close() }()
	return migrate(ctx, db, goose.UpContext)
}

func MigrateDown(ctx context.Context, pool *pgxpool.Pool) error {
	db := stdlib.OpenDBFromPool(pool)
	defer func() { _ = db.Close() }()
	return migrate(ctx, db, goose.DownContext)
}

func migrate(ctx context.Context, db *sql.DB, fn func(context.Context, *sql.DB, string, ...goose.OptionsFunc) error) error {
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("goose dialect: %w", err)
	}
	if err := fn(ctx, db, "."); err != nil {
		return fmt.Errorf("goose: %w", err)
	}
	return nil
}
