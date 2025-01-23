package main

import (
	"context"
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"log"
	"os"
	"os/signal"
	"service-template/application"
	"service-template/pkg"
	"syscall"

	"github.com/elastic/go-elasticsearch/v8"
)

func main() {
	logger := pkg.NewLogger(pkg.LoggerConfig{ServiceName: "elastic", LogPath: "logs/app.log"})
	father, cancel := context.WithCancel(context.Background())
	father = pkg.LoggerWithCtx(father, logger)
	defer cancel()

	logger.Info("config initializing")
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sig)

	go func() {
		<-sig
		logger.Info("Signal received. Shutting down server...")
		cancel()
	}()

	app := application.NewApp(logger)
	defer app.Stop()
	defer app.RegisterRecovers(logger, sig)()

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

	<-father.Done()
	logger.Info("application exited gracefully")
}
