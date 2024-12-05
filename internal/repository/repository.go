package repository

import "service-template/internal"

const SrvRepositoryService = "srv_repository"

type SrvRepository struct {
	locator internal.LocatorInterface
}

func NewSrvRepository() *SrvRepository {
	return &SrvRepository{}
}

func (repo *SrvRepository) SetServiceLocator(container internal.LocatorInterface) {
	repo.locator = container
}

func (repo *SrvRepository) GetServiceLocator() internal.LocatorInterface {
	return repo.locator
}
