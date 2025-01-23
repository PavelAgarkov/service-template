package repository

const RabbitRepositoryLabel = "rabbit_repository"

type RabbitRepository struct{}

func NewRabbitRepository() *RabbitRepository {
	return &RabbitRepository{}
}
