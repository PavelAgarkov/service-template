package pkg

import (
	"context"
	"fmt"
	"github.com/streadway/amqp"
	"golang.org/x/sync/errgroup"
	"log"
	"sync"
	"time"
)

const RabbitMqService = "rabbitmq"

type RabbitMQ struct {
	Conn           *amqp.Connection
	Channel        *amqp.Channel
	returnsChannel chan amqp.Return // Канал для возвратов
}

func NewRabbitMq(ctx context.Context, connectionString string, fallbackProducer func(ctx context.Context, ret amqp.Return) error) (*RabbitMQ, func()) {
	rmq := &RabbitMQ{
		returnsChannel: make(chan amqp.Return), // Инициализация канала возвратов
	}

	_, closer := rmq.build(connectionString)
	rmq.Channel.NotifyReturn(rmq.returnsChannel) // Установка канала возвратов
	ctx, cancel := context.WithCancel(ctx)

	// Запуск обработчика возвратов
	go rmq.handleReturns(ctx, fallbackProducer)

	return rmq, func() {
		cancel()
		closer() // Закрываем соединение и канал
	}
}

func (rmq *RabbitMQ) build(connectionString string) (*RabbitMQ, func()) {
	conn, err, rabbitCloser := rmq.connectToRabbitMQ(connectionString)
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

func (rmq *RabbitMQ) connectToRabbitMQ(url string) (*amqp.Connection, error, func()) {
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

func (rmq *RabbitMQ) Produce(
	queueName string,
	exchange string,
	message string,
	mandatory bool,
	immediate bool,
	contentType string,
) error {
	err := rmq.Channel.Publish(
		exchange, // имя обменника (пустая строка для отправки напрямую в очередь)
		// default exchange = "" (direct) означает, что сообщение будет отправлено в очередь с именем,
		//совпадающим с routingKey.
		queueName, // имя очереди
		mandatory, // обязательное сообщение false - сообщение будет удалено, если не будет доставлено
		immediate, // сообщение с временным TTL false - сообщение будет удалено, если не будет доставлено
		amqp.Publishing{
			ContentType: contentType, // "text/plain"
			Body:        []byte(message),
		})
	if err != nil {
		log.Printf("Не удалось отправить сообщение: %s", err)
		return fmt.Errorf("не удалось отправить сообщение: %s", err)
	}
	return nil
}

func (rmq *RabbitMQ) handleReturns(ctx context.Context, fallbackProducer func(ctx context.Context, ret amqp.Return) error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Паника во время обработки возвратов: %s", r)
		}
	}()
	log.Println("Запуск обработчика возвратов...")
	for {
		select {
		case <-ctx.Done():
			log.Println("Сигнал завершения. Остановка обработчика возвратов...")
			return
		case ret, ok := <-rmq.returnsChannel:
			if !ok {
				log.Println("Канал возвратов закрыт. Завершаем обработку возвратов.")
				return
			}

			// Обработка возврата
			g, ctx := errgroup.WithContext(ctx)
			g.SetLimit(1)
			g.Go(func() error {
				err := fallbackProducer(ctx, ret)
				if err != nil {
					return err
				}
				return nil
			})
			err := g.Wait()

			if err != nil {
				log.Printf("Не удалось обработать возврат: %s", err)
			}
			log.Printf("Сообщение не доставлено! Routing key: %s, Сообщение: %s", ret.RoutingKey, string(ret.Body))
		}
	}
}

func (rmq *RabbitMQ) Consumer(
	ctx context.Context,
	queueName string,
	handler func(ctx context.Context, msg amqp.Delivery) error,
	consumerName string,
	autoAsk bool,
	exclusive bool,
	noLocal bool,
	noWait bool,
	args amqp.Table,
) func() {
	// Получение сообщений из очереди
	msgs, err := rmq.Channel.Consume(
		queueName,    // имя очереди
		consumerName, // имя потребителя (пустая строка для автоматической генерации)
		autoAsk,      // автоматическое подтверждение
		exclusive,    // эксклюзивный доступ
		noLocal,      // нет ожидания
		noWait,       // локальное соединение
		args,         // дополнительные аргументы
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
				wg := sync.WaitGroup{}
				wg.Add(1)
				go func() {
					defer func() {
						if r := recover(); r != nil {
							log.Printf("Паника: %s", r)
							_ = msg.Nack(false, true)
							wg.Done()
						}
					}()
					defer wg.Done()
					err := handler(ctx, msg)
					if err != nil {
						log.Printf("Ошибка обработки сообщения: %s, ошибка: %v", msg.Body, err)
						//	//multiple = false:
						//	//	Отправляется Nack только для текущего сообщения. Это полезно, если вы хотите отклонить конкретное сообщение, а не все сообщения, которые находятся в ожидании подтверждения.
						//	//requeue:
						//	//	Если requeue = true, сообщение будет возвращено в очередь для повторной обработки другим (или тем же) потребителем.
						//	//	Если requeue = false, сообщение удаляется из очереди и больше не будет доставлено.
						err := msg.Nack(false, true)
						if err != nil {
							log.Printf("Не удалось вернуть сообщение в очередь: %s", err)
						}
					} else {
						err := msg.Ack(false)
						if err != nil {
							log.Printf("Не удалось подтвердить сообщение: %s", err)
						}
					}
				}()
				wg.Wait()
			}
		}
	}()

	return func() {
		<-end
		fmt.Println("Завершение работы consumer")
		close(end)
	}
}

func (rmq *RabbitMQ) BatchConsumer(
	ctx context.Context,
	queueName string,
	handler func(ctx context.Context, msg amqp.Delivery) error,
	consumerName string,
	autoAsk bool,
	exclusive bool,
	noLocal bool,
	noWait bool,
	args amqp.Table,
	batchSize int,
) func() {
	msgs, err := rmq.Channel.Consume(
		queueName,    // имя очереди
		consumerName, // имя потребителя (пустая строка для автоматической генерации)
		autoAsk,      // автоматическое подтверждение
		exclusive,    // эксклюзивный доступ
		noLocal,      // нет ожидания
		noWait,       // локальное соединение
		args,         // дополнительные аргументы
	)
	if err != nil {
		log.Fatalf("Не удалось начать потребление сообщений: %s", err)
	}

	batch := make([]amqp.Delivery, 0, batchSize)
	batchMutex := &sync.Mutex{} // Мьютекс для защиты batch

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
				// Обработка оставшихся сообщений
				batchMutex.Lock()
				if len(batch) > 0 {
					rmq.processBatch(ctx, append([]amqp.Delivery(nil), batch...), handler)
					batch = batch[:0]
				}
				batchMutex.Unlock()
				end <- true
				return
			case msg, ok := <-msgs:
				if !ok {
					log.Println("Канал сообщений закрыт. Завершаем обработку batch.")
					return
				}

				batchMutex.Lock()
				batch = append(batch, msg)
				if len(batch) >= batchSize {
					currentBatch := append([]amqp.Delivery(nil), batch...) // Создание копии
					batch = batch[:0]                                      // Очистка batch
					batchMutex.Unlock()

					go func(batchCopy []amqp.Delivery) {
						defer func() {
							if r := recover(); r != nil {
								log.Printf("Паника: %s", r)
							}
						}()
						rmq.processBatch(ctx, batchCopy, handler)
					}(currentBatch)
				} else {
					batchMutex.Unlock()
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

func (rmq *RabbitMQ) processBatch(
	ctx context.Context,
	batch []amqp.Delivery,
	handler func(cxt context.Context, msg amqp.Delivery) error) {
	log.Printf("Обработка batch из %d сообщений", len(batch))
	for _, msg := range batch {
		log.Printf("Обработка сообщения: %s", msg.Body)
		err := handler(ctx, msg)
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
