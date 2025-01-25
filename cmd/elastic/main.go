package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/elastic/go-elasticsearch/v8"
	"go.uber.org/zap"
	"log"
	"service-template/application"
	"service-template/pkg"
)

func main() {
	logger := pkg.NewLogger(pkg.LoggerConfig{ServiceName: "elastic", LogPath: "logs/app.log"})

	father, cancel := context.WithCancel(context.Background())
	father = pkg.LoggerWithCtx(father, logger)
	defer cancel()

	app := application.NewApp(father, nil, logger)
	app.Start(cancel)
	defer app.Stop()
	defer app.RegisterRecovers()()

	es, err := pkg.NewElasticFacade(
		elasticsearch.Config{
			Addresses: []string{"http://localhost:9200"},
			Username:  "elastic",
			Password:  "elasticpassword",
			// Если работаете по HTTPS/TLS — нужно добавить настройки для сертификатов (TLS), см. документацию.
		}, logger)
	if err != nil {
		logger.Fatal("Ошибка создания фасада Elasticsearch: %s", zap.Error(err))
	}

	// 1. Индексируем документ
	doc := map[string]interface{}{
		"title": "Hello from Go 2",
		"views": 47,
	}

	err = es.IndexDocument(father, "test-index", "2", doc)
	if err != nil {
		log.Fatalf("Ошибка при индексации: %s", err)
	}
	fmt.Println("Документ успешно проиндексирован.")

	// 2. Выполняем поиск
	searchQuery := map[string]interface{}{
		"match": map[string]interface{}{
			"title": "Go",
		},
	}

	m, err := es.Search(father, "test-index", searchQuery)
	if err != nil {
		log.Fatalf("Ошибка при выполнении поиска: %s", err)
	}

	// Логирование результатов поиска
	logger.Info("search result", zap.Any("result", m))

	// 3. Выполняем агрегацию
	aggregation := map[string]interface{}{
		"average_views": map[string]interface{}{
			"avg": map[string]interface{}{
				"field": "views",
			},
		},
	}

	aggResult, err := es.Aggregate(father, "test-index", aggregation)
	if err != nil {
		log.Fatalf("Ошибка при выполнении агрегации: %s", err)
	}

	// Вывод результатов агрегации
	prettyAggJSON, err := json.MarshalIndent(aggResult, "", "  ")
	if err != nil {
		log.Fatalf("Ошибка форматирования JSON агрегации: %s", err)
	}
	fmt.Println("Результат агрегации:")
	fmt.Println(string(prettyAggJSON))

	app.Run()
}
