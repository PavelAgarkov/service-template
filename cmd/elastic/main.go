package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

func main() {
	// 1. Настраиваем соединение
	cfg := elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
		Username:  "elastic",
		Password:  "elasticpassword",
		// Если работаете по HTTPS/TLS — нужно добавить настройки для сертификатов (TLS), см. документацию.
	}

	es, err := elasticsearch.NewClient(cfg)
	if err != nil {
		log.Fatalf("Ошибка создания клиента: %s", err)
	}

	// 2. Индексируем документ
	docBody := `{"title": "Hello from Go", "views": 42}`

	req := esapi.IndexRequest{
		Index:      "test-index",               // Название индекса
		DocumentID: "1",                        // Опционально можем указать ID
		Body:       strings.NewReader(docBody), // Сам JSON документа
		Refresh:    "true",                     // Обновить индекс сразу, чтобы поиск видел изменения
	}

	res, err := req.Do(context.Background(), es)
	if err != nil {
		log.Fatalf("Ошибка при индексации: %s", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		log.Fatalf("Elasticsearch вернул ошибку индексации: %s", res.String())
	}
	fmt.Println("Документ успешно проиндексирован.")

	// 3. Выполняем поиск
	searchBody := `{
	  "query": {
	    "match": {
	      "title": "Go"
	    }
	  }
	}`

	searchRes, err := es.Search(
		es.Search.WithContext(context.Background()),
		es.Search.WithIndex("test-index"),
		es.Search.WithBody(strings.NewReader(searchBody)),
		es.Search.WithTrackTotalHits(true),
		es.Search.WithPretty(), // форматированный JSON-ответ
	)
	if err != nil {
		log.Fatalf("Ошибка при поиске: %s", err)
	}
	defer searchRes.Body.Close()

	if searchRes.IsError() {
		log.Fatalf("Ошибка в поисковом запросе: %s", searchRes.String())
	}

	fmt.Println("Результат поиска:")
	fmt.Println(searchRes)
}
