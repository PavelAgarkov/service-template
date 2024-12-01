package handler

import (
	"net/http"
	"service-template/pkg"
)

type EmptyRequest struct {
	Name  string `json:"name"`
	Age   int    `json:"age"`
	Email string `json:"email"`
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

	json, err := serializer.Serialize(empty)

	serializer.ResponseJson(w, json, http.StatusOK)
	return
}
