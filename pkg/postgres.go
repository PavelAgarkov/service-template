package pkg

import (
	"log"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

const PostgresService = "postgres"

type PostgresRepository struct {
	db *sqlx.DB
}

func NewPostgres(host, user, dbname, password, sslMode string) (*PostgresRepository, func()) {
	dataSourceName := "host=" + host + " user=" + user + " dbname=" + dbname + " sslmode=" + sslMode + " password=" + password
	db, err := sqlx.Connect("postgres", dataSourceName)
	if err != nil {
		log.Fatalln(err)
	}
	r := &PostgresRepository{
		db: db,
	}
	return r, r.Shutdown()
}

func (r *PostgresRepository) Shutdown() func() {
	return func() {
		if err := r.db.Close(); err != nil {
			log.Fatalln(err)
		}
	}
}

func (r *PostgresRepository) GetDB() *sqlx.DB {
	return r.db
}
