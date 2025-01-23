package http_handler

import (
	"errors"
	"fmt"
	"github.com/go-playground/validator/v10"
	"go.etcd.io/etcd/client/v3/concurrency"
	"net/http"
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
	ctx := r.Context()

	l := pkg.LoggerFromCtx(ctx)

	postgres := h.postgres
	etcd := h.etcd
	serializer := h.serializer
	mutex := concurrency.NewMutex(etcd.GetSession(), "/distributed-lock/")

	l.Info("EmptyHandler before lock")
	if err := mutex.Lock(ctx); err != nil {
		serializer.ResponseJson(w, []byte(err.Error()), http.StatusInternalServerError)
		return
	}
	l.Info("EmptyHandler after lock")

	l.Info("EmptyHandler before unlock")
	if err := mutex.Unlock(ctx); err != nil {
		serializer.ResponseJson(w, []byte(err.Error()), http.StatusInternalServerError)
		return
	}
	l.Info("EmptyHandler after unlock")

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

	//rmq := srv.GetServiceLocator().Get(pkg.RabbitMqService).(*pkg.RabbitMQ)
	//err = rmq.Produce("1234", "", "message", true, false, "text/plain")
	if err != nil {
		l.Error(err.Error())
	}
	json, err := serializer.Serialize(empty)

	serializer.ResponseJson(w, json, http.StatusOK)
	return
}
