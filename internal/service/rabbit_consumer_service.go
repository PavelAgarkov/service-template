package service

import (
	"context"
	"encoding/json"
	amqp "github.com/rabbitmq/amqp091-go"
	gorabbitmq "github.com/wagslane/go-rabbitmq"
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
	locator internal.LocatorInterface
	router  map[string]*RabbitConsumeRoute
}

func NewConsumerRabbitService() *ConsumerRabbitService {
	return &ConsumerRabbitService{}
}

func (bs *ConsumerRabbitService) SetServiceLocator(container internal.LocatorInterface) {
	bs.locator = container
}

func (bs *ConsumerRabbitService) GetServiceLocator() internal.LocatorInterface {
	return bs.locator
}

func (bs *ConsumerRabbitService) Run(father context.Context, router map[string]*RabbitConsumeRoute) {
	for k, route := range router {
		go func(k string, route *RabbitConsumeRoute) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Recovered in f %v", r)
				}
			}()
			if err := route.Consumer.Run(route.Handler); err != nil {
				log.Printf("consumer error: %v", err)
			}
		}(k, route)
	}
}

func (bs *ConsumerRabbitService) HandleFailedMessageFromRabbitServer(ctx context.Context, ret gorabbitmq.Return) func() error {
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

		postgres := bs.GetServiceLocator().Get(pkg.PostgresService).(*pkg.PostgresRepository)
		rows, err := postgres.GetDB().NamedQuery("INSERT INTO rabbit_returns (data) VALUES (:data);", map[string]interface{}{
			"data": jsonData,
		})
		defer rows.Close()

		log.Printf("succes did record: %v", ret)
		return nil
	}
}
