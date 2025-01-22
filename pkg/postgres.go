package pkg

import (
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

const PostgresService = "postgres"

type PostgresRepository struct {
	db           *sqlx.DB
	ShutdownFunc func()
	logger       *zap.Logger
}

func NewPostgres(logger *zap.Logger, host, port, user, password, dbname, sslMode string) *PostgresRepository {
	dataSourceName := "host=" + host + " port=" + port + " user=" + user + " password=" + password + " dbname=" + dbname + " sslmode=" + sslMode
	db, err := sqlx.Connect("postgres", dataSourceName)
	if err != nil {
		logger.Fatal(err.Error())
	}
	r := &PostgresRepository{
		db:     db,
		logger: logger,
	}
	r.ShutdownFunc = r.Shutdown()
	return r
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
