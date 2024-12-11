package service

import (
	"context"
	"github.com/streadway/amqp"
	"service-template/pkg"
)

func (bs *ConsumerRabbitService) BlankConsumer() func(ctx context.Context, msg amqp.Delivery) error {
	return func(ctx context.Context, msg amqp.Delivery) error {
		postgres := bs.GetServiceLocator().Get(pkg.PostgresService).(*pkg.PostgresRepository)
		l := pkg.LoggerFromCtx(ctx)

		row := postgres.GetDB().QueryRow("insert into user_p(id) values (1);")
		if err := row.Err(); err != nil {
			l.Error(err.Error())
		}
		row1 := postgres.GetDB().QueryRow("select count(*) from user_p")
		if err := row1.Err(); err != nil {
			l.Error(err.Error())
		}
		a := 0
		row1.Scan(&a)
		return nil
	}
}
