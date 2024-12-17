package rabbit_migrations

import (
	amqp091 "github.com/rabbitmq/amqp091-go"
	"log"
)

const RabbitMqVersion202412170001 = 202412170001

//RoutingKey обязателен для обменников типа direct и topic.

func Up202412170001() func(ch *amqp091.Channel) (string, error) {
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
		log.Printf("Очередь '%s' успешно создана", queueName)

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
		log.Printf("Очередь '%s' успешно создана", queueName)

		return "Create RabbitMQ queue", nil
	}
}
