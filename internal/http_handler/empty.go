package http_handler

import (
	"fmt"
	"net/http"
	"service-template/internal/repository"
	"service-template/internal/service"
	"service-template/pkg"
)

type EmptyRequest struct {
	Name  string `json:"name"`
	Age   int    `json:"age"`
	Email string `json:"email"`
}

// EmptyHandler handles the empty endpoint.
// @Summary Empty endpoint
// @Description Handles an empty request and returns a response
// @Tags empty
// @Accept json
// @Produce json
// @Param empty body EmptyRequest true "Empty request"
// @Success 200 {object} EmptyRequest
// @Failure 400 {string} string "Bad Request"
// @Failure 408 {string} string "Request Timeout"
// @Router /empty [post]
func (h *Handlers) EmptyHandler(w http.ResponseWriter, r *http.Request) {

	//vars := mux.Vars(r)
	//id := vars["id"]

	//serializer := h.Container().Get("serializer").(*pkg.Serializer)
	//postgres := h.Container().Get("postgres").(*pkg.PostgresRepository)
	srv := h.Container().Get(service.ServiceSrv).(*service.Srv)
	serializer := srv.GetServiceLocator().Get(pkg.SerializerService).(*pkg.Serializer)
	repo := srv.GetServiceLocator().Get(repository.SrvRepositoryService).(*repository.SrvRepository)
	postgres := repo.GetServiceLocator().Get(pkg.PostgresService).(*pkg.PostgresRepository)

	fmt.Println(srv)

	ctx := r.Context()

	empty := &EmptyRequest{}
	err := serializer.Deserialize(r, empty)
	if err != nil {
		serializer.ResponseJson(w, []byte(err.Error()), http.StatusBadRequest)
		return
	}
	//time.Sleep(15 * time.Second)
	if ctx.Err() != nil {
		serializer.ResponseJson(w, []byte(ctx.Err().Error()), http.StatusRequestTimeout)
		return
	}
	row := postgres.GetDB().QueryRow("insert into user_p(id) values (1);")
	err = row.Err()
	fmt.Println(err)
	row1 := postgres.GetDB().QueryRow("select count(*) from user_p")
	err1 := row1.Err()
	fmt.Println(err1)
	a := 0
	row1.Scan(&a)
	fmt.Println(a)

	empty.Age = a

	json, err := serializer.Serialize(empty)

	serializer.ResponseJson(w, json, http.StatusOK)
	return
}
