package handler

import (
	"flick/pkg"
	"fmt"
	"net/http"
)

type EmptyRequest struct {
	Name  string `json:"name"`
	Age   int    `json:"age"`
	Email string `json:"email"`
}

func (h *Handlers) EmptyHandler(w http.ResponseWriter, r *http.Request) {
	serializer := h.Container().Get("serializer").(*pkg.Serializer)
	ctx := r.Context()
	fmt.Println(h.simpleService.Get())
	empty := &EmptyRequest{}
	err := serializer.Deserialize(r, empty)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
	if ctx.Err() != nil {
		return
	}
	//time.Sleep(10 * time.Second)
	json, err := serializer.Serialize(empty)

	w.WriteHeader(http.StatusOK)
	w.Write(json)
	//return response.Json(w, http.StatusOK, empty)
	fmt.Println(json)
}
