package repository

const PostgresRepositoryLabel = "postgres_repository"

type PostgresRepository struct {
}

func NewPostgresRepository() *PostgresRepository {
	return &PostgresRepository{}
}
