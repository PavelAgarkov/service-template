package pkg

import (
	"log"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type PostgresRepository struct {
	db *sqlx.DB
}

func NewPostgres(host, port, user, password, dbname, sslMode string) (*PostgresRepository, func()) {
	dataSourceName := "host=" + host + " port=" + port + " user=" + user + " password=" + password + " dbname=" + dbname + " sslmode=" + sslMode
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
