package handler

import (
	"errors"
	"github.com/go-playground/validator/v10"
	"net/http"
	"service-template/pkg"
)

type EmptyRequest struct {
	Name  string `json:"name" validate:"required,min=5,max=20"`
	Age   int    `json:"age" validate:"omitempty,gte=18,lte=120"`
	Email string `json:"email" validate:"required,email"`
}

func (h *Handlers) EmptyHandler(w http.ResponseWriter, r *http.Request) {
	serializer := h.Container().Get("serializer").(*pkg.Serializer)
	ctx := r.Context()

	h.simpleService.Get()

	empty := &EmptyRequest{}
	err := serializer.Deserialize(r, empty)
	if err != nil {
		serializer.ResponseJson(w, []byte(err.Error()), http.StatusBadRequest)
		return
	}
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

	json, err := serializer.Serialize(empty)

	serializer.ResponseJson(w, json, http.StatusOK)
	return
}
