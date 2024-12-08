package http_handler

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"net/http"
	"net/http/httptest"
	"service-template/internal"
	"service-template/internal/repository"
	"service-template/internal/service"
	"service-template/pkg"
	"testing"
)

type DBConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	Database string
}

func TestEmptyHandler(t *testing.T) {
	reqBody, err := json.Marshal(map[string]string{
		"name":  "Alice",
		"email": "alice@example.com",
	})
	assert.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "/", bytes.NewBuffer(reqBody))
	assert.NoError(t, err)

	ctx := context.Background()
	cfg, tearDown := setupPostgresContainer(t, ctx)
	defer tearDown()
	postgres, _ := pkg.NewPostgres(cfg.Host, cfg.Port, cfg.Username, cfg.Password, cfg.Database, "disable")

	container := internal.NewContainer(
		&internal.ServiceInit{Name: pkg.SerializerService, Service: pkg.NewSerializer()},
		&internal.ServiceInit{Name: pkg.PostgresService, Service: postgres},
	).
		Set(repository.SrvRepositoryService, repository.NewSrvRepository(), pkg.PostgresService).
		Set(service.ServiceSrv, service.NewSrv(), pkg.SerializerService, repository.SrvRepositoryService)

	handlers := NewHandlers(container)

	rr := httptest.NewRecorder()
	h := http.HandlerFunc(handlers.EmptyHandler)

	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	expected := `{"age":0, "email":"alice@example.com", "name":"Alice"}`
	assert.JSONEq(t, expected, rr.Body.String())
}

func setupPostgresContainer(t *testing.T, ctx context.Context) (*DBConfig, func()) {
	req := testcontainers.ContainerRequest{
		Image:        "postgres:latest",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "testuser",
			"POSTGRES_PASSWORD": "testpassword",
			"POSTGRES_DB":       "testdb",
		},
		WaitingFor: wait.ForListeningPort("5432/tcp"),
	}

	postgresC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	assert.NoError(t, err)
	host, err := postgresC.Host(ctx)
	assert.NoError(t, err)
	port, err := postgresC.MappedPort(ctx, "5432")
	assert.NoError(t, err)
	cfg := DBConfig{
		Host:     host,
		Port:     port.Port(),
		Username: "testuser",
		Password: "testpassword",
		Database: "testdb",
	}
	return &cfg, func() {
		testcontainers.CleanupContainer(t, postgresC)
	}
}
