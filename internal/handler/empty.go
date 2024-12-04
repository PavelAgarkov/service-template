package handler

import (
	"fmt"
	"errors"
	"github.com/go-playground/validator/v10"
	"net/http"
	"service-template/internal/service"
	"service-template/pkg"
)

type EmptyRequest struct {
	Name  string `json:"name" validate:"required,min=5,max=20"`
	Age   int    `json:"age" validate:"omitempty,gte=18,lte=120"`
	Email string `json:"email" validate:"required,email"`
}

func (h *Handlers) EmptyHandler(w http.ResponseWriter, r *http.Request) {
	//serializer := h.Container().Get("serializer").(*pkg.Serializer)
	postgres := h.Container().Get("postgres").(*pkg.PostgresRepository)
	simple := h.Container().Get("service.simple").(*service.Srv)
	serializer := simple.GetServiceLocator().Get("serializer").(*pkg.Serializer)

	fmt.Println(simple)

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
	validate := validator.New(validator.WithRequiredStructEnabled())
	err = validate.Struct(empty)
	if err != nil {
		var errs validator.ValidationErrors
		errors.As(err, &errs)
		serializer.ResponseJson(w, []byte(errs.Error()), http.StatusBadRequest)
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
