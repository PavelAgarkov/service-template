package pkg

import (
	gorabbitmq "github.com/wagslane/go-rabbitmq"
	"log"
)

const RabbitMqService = "rabbitmq"

type RabbitMQ struct {
}

func NewRabbitMQ() *RabbitMQ {
	return &RabbitMQ{}
}

func (r *RabbitMQ) RegisterPublisher(
	conn *gorabbitmq.Conn,
	returnHandler func(r gorabbitmq.Return),
	publishHandler func(c gorabbitmq.Confirmation),
	optionFuncs ...func(*gorabbitmq.PublisherOptions),
) *gorabbitmq.Publisher {
	publisher, err := gorabbitmq.NewPublisher(conn, optionFuncs...)
	if err != nil {
		log.Fatalf("failed to create publisher: %v", err)
	}
	publisher.NotifyReturn(returnHandler)
	publisher.NotifyPublish(publishHandler)

	return publisher
}

func (r *RabbitMQ) RegisterConsumer(
	conn *gorabbitmq.Conn,
	queue string,
	optionFuncs ...func(*gorabbitmq.ConsumerOptions),
) *gorabbitmq.Consumer {
	consumer, err := gorabbitmq.NewConsumer(conn, queue, optionFuncs...)
	if err != nil {
		log.Fatalf("failed to create consumer1: %v", err)
	}

	return consumer
}
