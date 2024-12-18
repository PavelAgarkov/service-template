package service

import (
	"context"
	amqp "github.com/rabbitmq/amqp091-go"
	gorabbitmq "github.com/wagslane/go-rabbitmq"
	"log"
	"service-template/internal"
)

const (
	BackgroundRabbit = "background_rabbit"
	Publisher        = "publisher"
	Publisher1       = "publisher1"
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
