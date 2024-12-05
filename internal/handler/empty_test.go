package handler

import (
	"bytes"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"service-template/internal"
	"service-template/pkg"
	"testing"
)

func TestEmptyHandler(t *testing.T) {
	reqBody, err := json.Marshal(map[string]string{
		"name":  "Alice",
		"email": "alice@example.com",
	})
	assert.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "/", bytes.NewBuffer(reqBody))
	assert.NoError(t, err)

	container := internal.NewContainer().
		Set("serializer", pkg.NewSerializer()).
		Set("postgres", nil).
		Set("service.simple", nil)
	handlers := NewHandlers(container)

	rr := httptest.NewRecorder()
	h := http.HandlerFunc(handlers.EmptyHandler)

	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	expected := `{"age":0, "email":"alice@example.com", "name":"Alice"}`
	assert.JSONEq(t, expected, rr.Body.String())
}
