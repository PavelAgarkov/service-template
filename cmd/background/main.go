package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"service-template/application"
	"service-template/config"
	"service-template/pkg"
	"syscall"
	"time"

	"github.com/streadway/amqp"
)

type RabbitMQ struct {
	Conn    *amqp.Connection
	Channel *amqp.Channel
}

func NewRabbitMq(connectionString string) (*RabbitMQ, func()) {
	rmq := &RabbitMQ{}
	_, closer := rmq.build(connectionString)
	return rmq, closer
}

//func (rmq *RabbitMQ) build(connectionString string) (*RabbitMQ, func()) {
//	conn, err, rabbitCloser := connectToRabbitMQ(connectionString)
//	if err != nil {
//		log.Fatalf("Не удалось подключиться к RabbitMQ: %s", err)
//	}
//	rmq.Conn = conn
//	rmq.Channel, err = rmq.Conn.Channel()
//	if err != nil {
//		log.Fatalf("Не удалось открыть канал: %s", err)
//	}
//
//	return rmq, func() {
//		rabbitCloser()
//		err := rmq.Channel.Close()
//		if err != nil {
//			log.Printf("Не удалось закрыть канал: %s", err)
//		}
//	}
//}

func (rmq *RabbitMQ) build(connectionString string) (*RabbitMQ, func()) {
	conn, err, rabbitCloser := connectToRabbitMQ(connectionString)
	if err != nil {
		log.Fatalf("Не удалось подключиться к RabbitMQ: %s", err)
	}
	rmq.Conn = conn
	rmq.Channel, err = rmq.Conn.Channel()
	if err != nil {
		log.Fatalf("Не удалось открыть канал: %s", err)
	}

	return rmq, func() {
		// Проверка и закрытие канала
		if rmq.Channel != nil {
			err := rmq.Channel.Close()
			if err != nil {
				log.Printf("Не удалось закрыть канал: %s", err)
			} else {
				log.Println("Канал успешно закрыт.")
			}
		}

		// Закрытие соединения
		rabbitCloser()
	}
}

func main() {
	father, cancel := context.WithCancel(context.Background())
	father = pkg.LoggerWithCtx(father, pkg.GetLogger())
	defer cancel()

	pkg.LoggerFromCtx(father).Info("config initializing")
	cfg := config.GetConfig()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sig)

	go func() {
		<-sig
		pkg.LoggerFromCtx(father).Info("Signal received. Shutting down server...")
		cancel()
	}()

	app := application.NewApp()
	defer func() {
		app.Stop()
		pkg.LoggerFromCtx(father).Info("app is stopped")
	}()

	postgres, postgresShutdown := pkg.NewPostgres(cfg.DB.Host, cfg.DB.Port, cfg.DB.Username, cfg.DB.Password, cfg.DB.Database, "disable")
	app.RegisterShutdown("postgres", postgresShutdown, 100)

	pkg.NewMigrations(postgres.GetDB().DB).Migrate("./migrations")

	//container := internal.NewContainer(
	//	&internal.ServiceInit{Name: pkg.SerializerService, Service: pkg.NewSerializer()},
	//	&internal.ServiceInit{Name: pkg.PostgresService, Service: postgres},
	//).
	//	Set(repository.SrvRepositoryService, repository.NewSrvRepository(), pkg.PostgresService).
	//	Set(service.ServiceSrv, service.NewSrv(), pkg.SerializerService, repository.SrvRepositoryService)

	rmq, rmqCloser := NewRabbitMq("amqp://user:password@localhost:5672/")
	app.RegisterShutdown("rabbitmq_server", rmqCloser, 50)

	// Объявление очереди
	queue, err := rmq.Channel.QueueDeclare(
		"test_queue", // имя очереди
		true,         // сохранять сообщения на диске
		false,        // удалять очередь при отсутствии подписчиков
		false,        // эксклюзивная очередь
		false,        // ждать подтверждения
		nil,          // дополнительные аргументы
	)
	if err != nil {
		log.Fatalf("Не удалось объявить очередь: %s", err)
	}

	// Отправка сообщения
	message := "Hello, RabbitMQ!"
	err = producer(rmq.Channel, &queue, message)
	if err != nil {
		log.Fatalf("Не удалось отправить сообщение: %s", err)
	}
	fmt.Printf("Сообщение отправлено в очередь %s: %s\n", queue.Name, message)

	//ctx, cancel := context.WithCancel(context.Background())
	//defer cancel()

	closerConsumer := consumer(father, rmq.Channel, &queue, func(msg amqp.Delivery) error { return nil })
	app.RegisterShutdown("rabbitmq_consumer", closerConsumer, 10)
	//closer := batchConsumer(ctx, rmq.Channel, &queue, func(msg amqp.Delivery) error { return nil })
	<-father.Done()

	//cancel()
	//closerConsumer()

	fmt.Println("end")
}

func producer(channel *amqp.Channel, queue *amqp.Queue, message string) error {
	err := channel.Publish(
		"",         // имя обменника (пустая строка для отправки напрямую в очередь)
		queue.Name, // имя очереди
		false,      // обязательное сообщение
		false,      // сообщение с временным TTL
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(message),
		})
	if err != nil {
		return fmt.Errorf("Не удалось отправить сообщение: %s", err)
	}
	return nil
}

//func connectToRabbitMQ(url string) (*amqp.Connection, error, func()) {
//	for {
//		conn, err := amqp.Dial(url)
//		if err != nil {
//			log.Printf("Ошибка подключения к RabbitMQ: %s. Повтор через 5 секунд...", err)
//			time.Sleep(5 * time.Second)
//			continue
//		}
//		log.Println("Успешно подключились к RabbitMQ!")
//		return conn, nil, func() {
//			err := conn.Close()
//			if err != nil {
//				log.Printf("Не удалось закрыть соединение: %s", err)
//			}
//		}
//	}
//}

func connectToRabbitMQ(url string) (*amqp.Connection, error, func()) {
	for {
		conn, err := amqp.Dial(url)
		if err != nil {
			log.Printf("Ошибка подключения к RabbitMQ: %s. Повтор через 5 секунд...", err)
			time.Sleep(5 * time.Second)
			continue
		}
		log.Println("Успешно подключились к RabbitMQ!")
		return conn, nil, func() {
			if conn != nil {
				err := conn.Close()
				if err != nil {
					log.Printf("Не удалось закрыть соединение: %s", err)
				} else {
					log.Println("Соединение с RabbitMQ успешно закрыто.")
				}
			}
		}
	}
}

func consumer(ctx context.Context, channel *amqp.Channel, queue *amqp.Queue, handler func(msg amqp.Delivery) error) func() {
	// Получение сообщений из очереди
	msgs, err := channel.Consume(
		queue.Name, // имя очереди
		"",         // имя потребителя (пустая строка для автоматической генерации)
		false,      // автоматическое подтверждение
		false,      // эксклюзивный доступ
		false,      // нет ожидания
		false,      // локальное соединение
		nil,        // дополнительные аргументы
	)
	if err != nil {
		log.Fatalf("Не удалось начать потребление сообщений: %s", err)
	}

	end := make(chan bool)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Паника: %s", r)
			}
		}()

		for {
			select {
			case <-ctx.Done():
				log.Println("Signal received. Shutting down consumer...")
				end <- true
				return
			case msg := <-msgs:
				log.Printf("Получено сообщение: %s", msg.Body)
				//если multiple = true, то подтверждается все сообщения до этого
				//если multiple = false, то подтверждается только текущее сообщение
				err := handler(msg)
				if err != nil {
					log.Printf("Ошибка обработки сообщения: %s, ошибка: %v", msg.Body, err)
					err := msg.Nack(false, true)
					if err != nil {
						log.Printf("Не удалось вернуть сообщение в очередь: %s", err)
					}
				}
				err = msg.Ack(false)
				if err != nil {
					//multiple = false:
					//	Отправляется Nack только для текущего сообщения. Это полезно, если вы хотите отклонить конкретное сообщение, а не все сообщения, которые находятся в ожидании подтверждения.
					//requeue:
					//	Если requeue = true, сообщение будет возвращено в очередь для повторной обработки другим (или тем же) потребителем.
					//	Если requeue = false, сообщение удаляется из очереди и больше не будет доставлено.
					err := msg.Nack(false, true)
					if err != nil {
						return
					}
					log.Printf("Не удалось подтвердить сообщение: %s", err)
				}
			}
		}
	}()

	return func() {
		<-end
		fmt.Println("Завершение работы consumer")
		close(end)
	}
}

func batchConsumer(ctx context.Context, channel *amqp.Channel, queue *amqp.Queue, handler func(msg amqp.Delivery) error) func() {
	msgs, err := channel.Consume(
		queue.Name, // имя очереди
		"",         // имя потребителя (пустая строка для автоматической генерации)
		false,      // автоматическое подтверждение
		false,      // эксклюзивный доступ
		false,      // нет ожидания
		false,      // локальное соединение
		nil,        // дополнительные аргументы
	)
	if err != nil {
		log.Fatalf("Не удалось начать потребление сообщений: %s", err)
	}

	batchSize := 5
	batch := make([]amqp.Delivery, 0, batchSize)

	end := make(chan bool)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Паника: %s", r)
			}
		}()
		for {
			select {
			case <-ctx.Done():
				log.Println("Сигнал завершения. Остановка batchConsumer...")
				if len(batch) > 0 {
					log.Println("Обработка оставшихся сообщений в batch...")
					processBatch(batch, handler)
					for _, m := range batch {
						if err := m.Ack(false); err != nil {
							log.Printf("Ошибка подтверждения сообщения: %s", err)
						}
					}
				}
				end <- true
				return
			case msg, ok := <-msgs:
				if !ok {
					log.Println("Канал сообщений закрыт. Завершаем обработку batch.")
					return
				}

				batch = append(batch, msg)
				if len(batch) >= batchSize {
					processBatch(batch, handler)
					for _, m := range batch {
						if err := m.Ack(false); err != nil {
							log.Printf("Ошибка подтверждения сообщения: %s", err)
						}
					}
					batch = batch[:0] // Очистка batch
				}
			}
		}
	}()

	return func() {
		<-end
		fmt.Println("Завершение работы batchConsumer")
		close(end)
	}
}

func processBatch(batch []amqp.Delivery, handler func(msg amqp.Delivery) error) {
	log.Printf("Обработка batch из %d сообщений", len(batch))
	for _, msg := range batch {
		log.Printf("Обработка сообщения: %s", msg.Body)
		err := handler(msg)
		if err != nil {
			log.Printf("Ошибка обработки сообщения: %s, ошибка: %v", msg.Body, err)
			if nackErr := msg.Nack(false, true); nackErr != nil {
				log.Printf("Ошибка при возврате сообщения в очередь: %s", nackErr)
			}
		} else {
			if ackErr := msg.Ack(false); ackErr != nil {
				log.Printf("Ошибка подтверждения сообщения: %s", ackErr)
			}
		}
	}
}
