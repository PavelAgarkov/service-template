package pkg

import (
	"encoding/json"
	"io"
	"net/http"
)

type Serializer struct{}

func NewSerializer() *Serializer {
	return &Serializer{}
}

func (serializer *Serializer) Deserialize(r *http.Request, object any) error {
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(body, &object); err != nil {
		return err
	}
	return nil
}

func (serializer *Serializer) Serialize(object any) ([]byte, error) {
	jsonData, err := json.Marshal(object)
	if err != nil {
		return nil, err
	}

	return jsonData, nil
}
