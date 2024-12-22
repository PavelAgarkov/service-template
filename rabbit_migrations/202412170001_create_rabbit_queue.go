package rabbit_migrations

import (
	"fmt"
	amqp091 "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

const RabbitMqVersion202412170001 = 202412170001

//RoutingKey обязателен для обменников типа direct и topic.

func Up202412170001(logger *zap.Logger) func(ch *amqp091.Channel) (string, error) {
	return func(ch *amqp091.Channel) (string, error) {
		queueName := "my_queue"
		_, err := ch.QueueDeclare(queueName, true, false, false, false, nil)
		if err != nil {
			return "", err
		}
		err = ch.ExchangeDeclare("my_events", "direct", true, false, false, false, nil)
		if err != nil {
			return "", err
		}
		err = ch.QueueBind(queueName, queueName, "my_events", false, nil)
		if err != nil {
			return "", err
		}
		logger.Info(fmt.Sprintf("Очередь '%s' успешно создана", queueName))

		queueName = "test_queue"
		_, err = ch.QueueDeclare(queueName, true, false, false, false, nil)
		if err != nil {
			return "", err
		}
		err = ch.ExchangeDeclare("events", "direct", true, false, false, false, nil)
		if err != nil {
			return "", err
		}
		err = ch.QueueBind(queueName, queueName, "events", false, nil)
		if err != nil {
			return "", err
		}
		logger.Info(fmt.Sprintf("Очередь '%s' успешно создана", queueName))

		return "Create RabbitMQ queue", nil
	}
}
