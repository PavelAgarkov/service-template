package pkg

import (
	"database/sql"
	"fmt"
	"github.com/pressly/goose/v3"
	"go.uber.org/zap"
)

type Migrations struct {
	db     *sql.DB
	logger *zap.Logger
}

func NewMigrations(db *sql.DB, logger *zap.Logger) *Migrations {
	return &Migrations{
		db:     db,
		logger: logger,
	}
}

func (m *Migrations) Migrate(path string, tableName string) *Migrations {
	goose.SetBaseFS(nil)
	err := goose.SetDialect("postgres")
	if err != nil {
		m.logger.Fatal(fmt.Sprintf("error set dialect: %v", err))
	}
	goose.SetTableName(tableName)

	if err := goose.Up(m.db, path); err != nil {
		m.logger.Fatal(fmt.Sprintf("Ошибка при выполнении миграции: %v", err))
	}

	return m
}
