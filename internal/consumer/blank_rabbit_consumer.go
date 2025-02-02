package consumer

import (
	"context"
	"fmt"
	gorabbitmq "github.com/wagslane/go-rabbitmq"
	"service-template/pkg"
)

func (bs *ConsumerRabbitService) BlankConsumer(ctx context.Context) func(d gorabbitmq.Delivery) gorabbitmq.Action {
	return func(d gorabbitmq.Delivery) gorabbitmq.Action {
		l := pkg.LoggerFromCtx(ctx)

		//publisher1 := bs.GetServiceLocator().Get(Publisher1).(*gorabbitmq.Publisher)
		publisher1 := bs.publisher1
		confirms, err := publisher1.PublishWithDeferredConfirmWithContext(
			ctx,
			[]byte("publisher: hello, world"),
			[]string{"test_queue"},
			gorabbitmq.WithPublishOptionsContentType("application/json"),
			gorabbitmq.WithPublishOptionsExchange("events"),
			gorabbitmq.WithPublishOptionsMandatory,
			gorabbitmq.WithPublishOptionsPersistentDelivery,
		)
		if err != nil {
			l.Error(fmt.Sprintf("failed to publish: %v", err))
		}
		if len(confirms) == 0 || confirms[0] == nil {
			l.Info("message publishing not confirmed")
		}
		ok, err := confirms[0].WaitContext(context.Background())
		if err != nil {
			l.Error(err.Error())
		}
		if ok {
			l.Info("message publishing confirmed")
		} else {
			l.Error("message publishing not confirmed")
		}

		//time.Sleep(1 * time.Second)

		l.Info(fmt.Sprintf("consumed_0: %v", string(d.Body)))
		return gorabbitmq.Ack
	}
}
