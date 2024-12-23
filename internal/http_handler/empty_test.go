package http_handler

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"log"
	"net/http"
	"net/http/httptest"
	"service-template/internal"
	"service-template/internal/repository"
	"service-template/internal/service"
	"service-template/internal/testhelpers"
	"service-template/pkg"
	"testing"

	"github.com/stretchr/testify/suite"
)

type EmptyTestSuite struct {
	suite.Suite
	pgContainer *testhelpers.PostgresContainer
	ctx         context.Context
	container   *internal.Container
}

func (suite *EmptyTestSuite) SetupSuite() {
	suite.ctx = context.Background()
	pgContainer, err := testhelpers.CreatePostgresContainer(suite.ctx)
	if err != nil {
		log.Fatal(err)
	}
	suite.pgContainer = pgContainer

	logger := pkg.NewLogger("test", "logs/app.log")

	postgres, _ := pkg.NewPostgres(
		logger,
		pgContainer.DBConfig.Host,
		pgContainer.DBConfig.Port,
		pgContainer.DBConfig.Username,
		pgContainer.DBConfig.Password,
		pgContainer.DBConfig.Database,
		"disable",
	)

	container := internal.NewContainer(
		logger,
		&internal.ServiceInit{Name: pkg.SerializerService, Service: pkg.NewSerializer()},
		&internal.ServiceInit{Name: pkg.PostgresService, Service: postgres},
	).
		Set(repository.SrvRepositoryService, repository.NewSrvRepository(), pkg.PostgresService).
		Set(service.ServiceSrv, service.NewSrv(), pkg.SerializerService, repository.SrvRepositoryService)
	suite.container = container
}

func (suite *EmptyTestSuite) TearDownSuite() {
	if err := suite.pgContainer.Terminate(suite.ctx); err != nil {
		log.Fatalf("error terminating postgres container: %s", err)
	}
}

func (suite *EmptyTestSuite) TestSuiteEmptyHandler() {
	t := suite.T()
	reqBody, err := json.Marshal(map[string]string{
		"name":  "Alice",
		"email": "alice@example.com",
	})
	assert.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "/", bytes.NewBuffer(reqBody))

	assert.NoError(t, err)

	handlers := NewHandlers(suite.container)

	rr := httptest.NewRecorder()
	h := http.HandlerFunc(handlers.EmptyHandler)

	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	expected := `{"age":0, "email":"alice@example.com", "name":"Alice"}`
	assert.JSONEq(t, expected, rr.Body.String())
}

func TestEmptyTestSuite(t *testing.T) {
	suite.Run(t, new(EmptyTestSuite))
}
