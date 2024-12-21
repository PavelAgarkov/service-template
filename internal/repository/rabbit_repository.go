package repository

import "service-template/internal"

const RabbitRepositoryLabel = "rabbit_repository"

type RabbitRepository struct {
	locator internal.LocatorInterface
}

func NewRabbitRepository() *RabbitRepository {
	return &RabbitRepository{}
}

func (repo *RabbitRepository) SetServiceLocator(container internal.LocatorInterface) {
	repo.locator = container
}

func (repo *RabbitRepository) GetServiceLocator() internal.LocatorInterface {
	return repo.locator
}
