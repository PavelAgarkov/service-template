package service

import (
	"context"
	"github.com/streadway/amqp"
	"service-template/internal"
	"service-template/pkg"
)

type RabbitConsumers interface {
	BlankConsumer() func(ctx context.Context, msg amqp.Delivery) error
}

const BackgroundRabbit = "background_rabbit"

type consumer struct {
	name         string
	batch        bool
	shutdown     func()
	handler      func(ctx context.Context, message amqp.Delivery) error
	queue        *amqp.Queue
	consumerName string
	autoAsk      bool
	exclusive    bool
	noLocal      bool
	noWait       bool
	args         amqp.Table
}

type ConsumerRabbitService struct {
	consumers []*consumer
	locator   internal.LocatorInterface
}

func NewBackgroundService() *ConsumerRabbitService {
	return &ConsumerRabbitService{
		consumers: make([]*consumer, 0),
	}
}

func (bs *ConsumerRabbitService) RegisterRabbitQueue(
	name string,
	queue *amqp.Queue,
	handler func(ctx context.Context, message amqp.Delivery) error,
	batch bool,
	consumerName string,
	autoAsk bool,
	exclusive bool,
	noLocal bool,
	noWait bool,
	args amqp.Table,
) *ConsumerRabbitService {
	bs.consumers = append(bs.consumers, &consumer{
		name:         name,
		batch:        batch,
		handler:      handler,
		queue:        queue,
		consumerName: consumerName,
		autoAsk:      autoAsk,
		exclusive:    exclusive,
		noLocal:      noLocal,
		noWait:       noWait,
		args:         args,
		shutdown:     nil,
	})
	return bs
}

func (bs *ConsumerRabbitService) SetServiceLocator(container internal.LocatorInterface) {
	bs.locator = container
}

func (bs *ConsumerRabbitService) GetServiceLocator() internal.LocatorInterface {
	return bs.locator
}

func (bs *ConsumerRabbitService) RunConsumers(ctx context.Context) *ConsumerRabbitService {
	rmq := bs.GetServiceLocator().Get(pkg.RabbitMqService).(*pkg.RabbitMQ)
	for _, consumer := range bs.consumers {
		if consumer.batch {
			closer := rmq.BatchConsumer(
				ctx,
				consumer.queue.Name,
				consumer.handler,
				consumer.consumerName,
				consumer.autoAsk,
				consumer.exclusive,
				consumer.noLocal,
				consumer.noWait,
				consumer.args,
			)
			consumer.shutdown = closer
		} else {
			closer := rmq.Consumer(
				ctx,
				consumer.queue.Name,
				consumer.handler,
				consumer.consumerName,
				consumer.autoAsk,
				consumer.exclusive,
				consumer.noLocal,
				consumer.noWait,
				consumer.args,
			)
			consumer.shutdown = closer
		}
	}
	return bs
}

func (bs *ConsumerRabbitService) GetRegisteredShutdowns() map[string]func() {
	closers := make(map[string]func())
	for _, consumer := range bs.consumers {
		closers[consumer.name] = consumer.shutdown
	}
	return closers
}
