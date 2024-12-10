package http_handler

import (
	"errors"
	"fmt"
	"github.com/go-playground/validator/v10"
	"net/http"
	"service-template/internal/repository"
	"service-template/internal/service"
	"service-template/pkg"
)

type EmptyRequest struct {
	Name  string `json:"name" validate:"required,min=5,max=20"`
	Age   int    `json:"age" validate:"omitempty,gte=18,lte=120"`
	Email string `json:"email" validate:"required,email"`
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

	ctx := r.Context()

	l := pkg.LoggerFromCtx(ctx)

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
	validate := validator.New(validator.WithRequiredStructEnabled())
	err = validate.Struct(empty)
	if err != nil {
		var errs validator.ValidationErrors
		errors.As(err, &errs)
		serializer.ResponseJson(w, []byte(errs.Error()), http.StatusBadRequest)
		return
	}
	row := postgres.GetDB().QueryRow("insert into user_p(id) values (1);")
	if err = row.Err(); err != nil {
		l.Error(err.Error())
	}
	row1 := postgres.GetDB().QueryRow("select count(*) from user_p")
	if err = row1.Err(); err != nil {
		l.Error(err.Error())
	}
	a := 0
	row1.Scan(&a)
	l.Debug(fmt.Sprintf("%v", a))

	empty.Age = a

	json, err := serializer.Serialize(empty)

	serializer.ResponseJson(w, json, http.StatusOK)
	return
}
