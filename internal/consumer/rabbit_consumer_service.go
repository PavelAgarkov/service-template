package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	amqp "github.com/rabbitmq/amqp091-go"
	gorabbitmq "github.com/wagslane/go-rabbitmq"
	"go.uber.org/dig"
	"log"
	"service-template/internal"
	"service-template/pkg"
)

const (
	BackgroundRabbitConsumeService = "background_rabbit_consume_service"
	Publisher                      = "publisher"
	Publisher1                     = "publisher1"
)

type RabbitConsumeRoute struct {
	Consumer *gorabbitmq.Consumer
	Handler  gorabbitmq.Handler
}

type RabbitConsumers interface {
	BlankConsumer() func(ctx context.Context, msg amqp.Delivery) error
}

type ConsumerRabbitService struct {
	locator    internal.LocatorInterface
	postgres   *pkg.PostgresRepository
	publisher0 *gorabbitmq.Publisher
	publisher1 *gorabbitmq.Publisher
}

func NewConsumerRabbitService(dig *dig.Container) *ConsumerRabbitService {
	c := &ConsumerRabbitService{}

	err := dig.Invoke(func(p *pkg.PostgresRepository) {
		c.postgres = p
	})
	if err != nil {
		log.Fatal(err)
	}

	return c
}

func (bs *ConsumerRabbitService) SetPublishers(publisher, publisher1 *gorabbitmq.Publisher) {
	bs.publisher0 = publisher
	bs.publisher1 = publisher1
}

func (bs *ConsumerRabbitService) Run(father context.Context, router map[string]*RabbitConsumeRoute) {
	l := pkg.LoggerFromCtx(father)
	for k, route := range router {
		go func(k string, route *RabbitConsumeRoute) {
			defer func() {
				if r := recover(); r != nil {
					l.Info(fmt.Sprintf("Recovered in f %v", r))
				}
			}()
			if err := route.Consumer.Run(route.Handler); err != nil {
				l.Info(fmt.Sprintf("consumer error: %v", err))
			}
		}(k, route)
	}
}

func (bs *ConsumerRabbitService) HandleFailedMessageFromRabbitServer(father context.Context, ret gorabbitmq.Return) func() error {
	l := pkg.LoggerFromCtx(father)
	return func() error {
		newRet := &pkg.Return{
			ReplyCode:       ret.ReplyCode,
			ReplyText:       ret.ReplyText,
			Exchange:        ret.Exchange,
			RoutingKey:      ret.RoutingKey,
			ContentType:     ret.ContentType,
			ContentEncoding: ret.ContentEncoding,
			Headers:         ret.Headers,
			DeliveryMode:    ret.DeliveryMode,
			Priority:        ret.Priority,
			CorrelationId:   ret.CorrelationId,
			ReplyTo:         ret.ReplyTo,
			Expiration:      ret.Expiration,
			MessageId:       ret.MessageId,
			Timestamp:       ret.Timestamp,
			Type:            ret.Type,
			UserId:          ret.UserId,
			AppId:           ret.AppId,
			Body:            string(ret.Body),
		}

		jsonData, err := json.Marshal(newRet)
		if err != nil {
			return err
		}

		postgres := bs.postgres
		rows, err := postgres.GetDB().NamedQuery("INSERT INTO rabbit_returns (data) VALUES (:data);", map[string]interface{}{
			"data": jsonData,
		})
		defer rows.Close()

		l.Info(fmt.Sprintf("succes did record: %v", ret))
		return nil
	}
}
