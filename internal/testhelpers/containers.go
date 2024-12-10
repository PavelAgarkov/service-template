package testhelpers

import (
	"context"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"service-template/config"
)

type PostgresContainer struct {
	*postgres.PostgresContainer
	DBConfig config.DBConfig
}

func CreatePostgresContainer(ctx context.Context) (*PostgresContainer, error) {
	const (
		testUser     = "testuser"
		testPassword = "testpassword"
		testDB       = "testdb"
	)
	pgContainer, err := postgres.Run(ctx,
		"postgres:latest",
		postgres.WithUsername(testUser),
		postgres.WithPassword(testPassword),
		postgres.WithDatabase(testDB),
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("5432/tcp"),
		),
	)
	if err != nil {
		return nil, err
	}

	host, err := pgContainer.Host(ctx)
	if err != nil {
		return nil, err
	}
	port, err := pgContainer.MappedPort(ctx, "5432")
	if err != nil {
		return nil, err
	}
	return &PostgresContainer{
		PostgresContainer: pgContainer,
		DBConfig: config.DBConfig{
			Host:     host,
			Port:     port.Port(),
			Username: testUser,
			Password: testPassword,
			Database: testDB,
		},
	}, nil
}
