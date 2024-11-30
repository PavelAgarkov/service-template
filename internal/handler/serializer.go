package handler

import (
	"encoding/json"
	"io"
	"net/http"
)

func deserialize(r *http.Request, object any) error {
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, &object); err != nil {
		return err
	}
	return nil
}

func serialize(object any) ([]byte, error) {
	jsonData, err := json.Marshal(object)
	if err != nil {
		return nil, err
	}

	return jsonData, nil
}
