package pkg

import (
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

const PostgresService = "postgres"

type PostgresRepository struct {
	db     *sqlx.DB
	logger *zap.Logger
}

func NewPostgres(logger *zap.Logger, host, port, user, password, dbname, sslMode string) (*PostgresRepository, func()) {
	dataSourceName := "host=" + host + " port=" + port + " user=" + user + " password=" + password + " dbname=" + dbname + " sslmode=" + sslMode
	db, err := sqlx.Connect("postgres", dataSourceName)
	if err != nil {
		logger.Fatal(err.Error())
	}
	r := &PostgresRepository{
		db:     db,
		logger: logger,
	}
	return r, r.Shutdown()
}

func (r *PostgresRepository) Shutdown() func() {
	return func() {
		if err := r.db.Close(); err != nil {
			r.logger.Fatal(err.Error())
		}
	}
}

func (r *PostgresRepository) GetDB() *sqlx.DB {
	return r.db
}
