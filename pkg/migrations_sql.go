package pkg

import (
	"database/sql"
	"github.com/pressly/goose/v3"
	"log"
)

type Migrations struct {
	db *sql.DB
}

func NewMigrations(db *sql.DB) *Migrations {
	return &Migrations{
		db: db,
	}
}

func (m *Migrations) Migrate(path string, tableName string) *Migrations {
	goose.SetBaseFS(nil)
	err := goose.SetDialect("postgres")
	if err != nil {
		log.Fatalf("error set dialect: %v", err)
	}
	goose.SetTableName(tableName)

	if err := goose.Up(m.db, path); err != nil {
		log.Fatalf("Ошибка при выполнении миграции: %v", err)
	}

	return m
}
