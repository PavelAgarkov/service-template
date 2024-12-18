package pkg

import (
	amqp "github.com/rabbitmq/amqp091-go"
	gorabbitmq "github.com/wagslane/go-rabbitmq"
	"log"
	"time"
)

const RabbitMqService = "rabbitmq"

type Return struct {
	ReplyCode       uint16     `json:"reply_code"`
	ReplyText       string     `json:"reply_text"`
	Exchange        string     `json:"exchange"`
	RoutingKey      string     `json:"routing_key"`
	ContentType     string     `json:"content_type"`
	ContentEncoding string     `json:"content_encoding"`
	Headers         amqp.Table `json:"headers"`
	DeliveryMode    uint8      `json:"delivery_mode"`
	Priority        uint8      `json:"priority"`
	CorrelationId   string     `json:"correlation_id"`
	ReplyTo         string     `json:"reply_to"`
	Expiration      string     `json:"expiration"`
	MessageId       string     `json:"message_id"`
	Timestamp       time.Time  `json:"timestamp"`
	Type            string     `json:"type"`
	UserId          string     `json:"user_id"`
	AppId           string     `json:"app_id"`
	Body            string     `json:"body"`
}

type RabbitMQ struct{}

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
	// события, которые происходят при публикации сообщения с несуществующим routing key или exchange
	publisher.NotifyReturn(returnHandler)

	// события, которые происходят при подтверждении публикации сообщения
	// срабатывает в том числе при nack сообщения
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
