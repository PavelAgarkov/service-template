package repository

import "service-template/internal"

const PostgresRepositoryLabel = "postgres_repository"

type PostgresRepository struct {
	locator internal.LocatorInterface
}

func NewPostgresRepository() *PostgresRepository {
	return &PostgresRepository{}
}

func (repo *PostgresRepository) SetServiceLocator(container internal.LocatorInterface) {
	repo.locator = container
}

func (repo *PostgresRepository) GetServiceLocator() internal.LocatorInterface {
	return repo.locator
}
