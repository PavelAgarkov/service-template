package handler

import (
	"fmt"
	"net/http"
	"time"
)

type EmptyRequest struct {
	Name  string `json:"name"`
	Age   int    `json:"age"`
	Email string `json:"email"`
}

func EmptyHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	empty := &EmptyRequest{}
	err := deserialize(r, empty)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
	if ctx.Err() != nil {
		return
	}
	time.Sleep(10 * time.Second)
	w.WriteHeader(http.StatusOK)
	json, err := serialize(empty)

	w.Write(json)
	fmt.Println(json)
}
