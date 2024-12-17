package service

import (
	"context"
	"fmt"
	gorabbitmq "github.com/wagslane/go-rabbitmq"
	"service-template/pkg"
	"time"
)

func (bs *ConsumerRabbitService) BlankConsumer(ctx context.Context) func(d gorabbitmq.Delivery) gorabbitmq.Action {
	return func(d gorabbitmq.Delivery) gorabbitmq.Action {
		//postgres := bs.GetServiceLocator().Get(pkg.PostgresService).(*pkg.PostgresRepository)
		l := pkg.LoggerFromCtx(ctx)

		//row := postgres.GetDB().QueryRow("insert into user_p(id) values (1);")
		//if err := row.Err(); err != nil {
		//	l.Error(err.Error())
		//	return gorabbitmq.Ack
		//}
		//row1 := postgres.GetDB().QueryRow("select count(*) from user_p")
		//if err := row1.Err(); err != nil {
		//	l.Error(err.Error())
		//	return gorabbitmq.NackDiscard
		//}
		//a := 0
		//row1.Scan(&a)

		publisher1 := bs.GetServiceLocator().Get(Publisher1).(*gorabbitmq.Publisher)
		if err := publisher1.PublishWithContext(
			ctx,
			[]byte("publisher: hello, world"),
			[]string{"test_queue"},
			gorabbitmq.WithPublishOptionsContentType("application/json"),
			gorabbitmq.WithPublishOptionsExchange("events"),
			gorabbitmq.WithPublishOptionsMandatory,
			gorabbitmq.WithPublishOptionsPersistentDelivery,
		); err != nil {
			l.Error(fmt.Sprintf("failed to publish: %v", err))
		}

		time.Sleep(1 * time.Second)

		l.Info(fmt.Sprintf("consumed_0: %v", string(d.Body)))
		return gorabbitmq.Ack
	}
}
