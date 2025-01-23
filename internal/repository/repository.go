package repository

const SrvRepositoryService = "srv_repository"

type SrvRepository struct{}

func NewSrvRepository() *SrvRepository {
	return &SrvRepository{}
}
