package pkg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"go.uber.org/zap"
	"io"
)

type ElasticFacade struct {
	client *elasticsearch.Client
	logger *zap.Logger
}

func NewElasticFacade(cfg elasticsearch.Config, logger *zap.Logger) (*ElasticFacade, error) {
	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания клиента: %w", err)
	}

	res, err := client.Info()
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к Elasticsearch: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("ошибка ответа от Elasticsearch: %s", res.String())
	}

	logger.Info("Успешное подключение к Elasticsearch.")

	return &ElasticFacade{
		client: client,
		logger: logger,
	}, nil
}

// IndexDocument индексирует один документ
func (e *ElasticFacade) IndexDocument(ctx context.Context, index string, id string, doc interface{}) error {
	docJSON, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("ошибка маршалингу документа: %w", err)
	}

	req := esapi.IndexRequest{
		Index:      index,
		DocumentID: id,
		Body:       bytes.NewReader(docJSON),
		Refresh:    "true",
	}

	res, err := req.Do(ctx, e.client)
	if err != nil {
		return fmt.Errorf("ошибка при индексации: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		// Читаем тело ошибки
		bodyBytes, _ := io.ReadAll(res.Body)
		return fmt.Errorf("elasticsearch вернул ошибку индексации: %s", string(bodyBytes))
	}

	e.logger.Info(fmt.Sprintf("Документ %s успешно проиндексирован в индекс %s.\n", id, index))
	return nil
}

// Search выполняет поиск по заданному запросу
func (e *ElasticFacade) Search(ctx context.Context, index string, query interface{}) (map[string]interface{}, error) {
	// Включаем track_total_hits в тело запроса
	searchQuery := map[string]interface{}{
		"query":            query,
		"track_total_hits": true,
	}

	queryJSON, err := json.Marshal(searchQuery)
	if err != nil {
		return nil, fmt.Errorf("ошибка маршалингу поискового запроса: %w", err)
	}

	req := esapi.SearchRequest{
		Index: []string{index},
		Body:  bytes.NewReader(queryJSON),
		//Pretty: true,
	}

	res, err := req.Do(ctx, e.client)
	if err != nil {
		return nil, fmt.Errorf("ошибка при выполнении поиска: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		// Читаем тело ошибки
		bodyBytes, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("elasticsearch вернул ошибку при поиске: %s", string(bodyBytes))
	}

	var searchResult map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&searchResult); err != nil {
		return nil, fmt.Errorf("ошибка декодирования результатов поиска: %w", err)
	}

	e.logger.Info(fmt.Sprintf("Поиск по индексу %s выполнен успешно.\n", index))
	return searchResult, nil
}

// Aggregate выполняет агрегационный запрос
func (e *ElasticFacade) Aggregate(ctx context.Context, index string, aggregation interface{}) (map[string]interface{}, error) {
	// Структура запроса с агрегацией
	searchQuery := map[string]interface{}{
		"size": 0, // Не возвращаем документы, только агрегации
		"aggs": aggregation,
	}

	// Маршалинг запроса в JSON
	searchJSON, err := json.Marshal(searchQuery)
	if err != nil {
		return nil, fmt.Errorf("ошибка маршалингу агрегационного запроса: %w", err)
	}

	// Создание запроса SearchRequest
	req := esapi.SearchRequest{
		Index: []string{index},
		Body:  bytes.NewReader(searchJSON),
		//Pretty: true,
	}

	// Выполнение запроса
	res, err := req.Do(ctx, e.client)
	if err != nil {
		return nil, fmt.Errorf("ошибка при выполнении агрегации: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		// Читаем тело ошибки
		bodyBytes, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("elasticsearch вернул ошибку при выполнении агрегации: %s", string(bodyBytes))
	}

	// Чтение и декодирование ответа
	var aggResult map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&aggResult); err != nil {
		return nil, fmt.Errorf("ошибка декодирования результатов агрегации: %w", err)
	}

	e.logger.Info(fmt.Sprintf("Агрегация по индексу %s выполнена успешно.\n", index))
	return aggResult, nil
}
