package pkg

import (
	"context"
	"fmt"
	"github.com/rabbitmq/amqp091-go"
)

func (m *Migrations) set(ctx context.Context, version int, description string) error {
	_, err := m.db.ExecContext(ctx, `
        INSERT INTO rabbit_migrations (version_id, description, is_applied)
        VALUES ($1, $2, TRUE) ON CONFLICT (version_id) DO NOTHING
    `, version, description)
	if err != nil {
		m.logger.Info(fmt.Sprintf("Ошибка при фиксации миграции: %v", err))
		return err
	}
	return nil
}

func (m *Migrations) applyCustom(
	ctx context.Context,
	migration func(ch *amqp091.Channel) (string, error),
	version int,
	connections []*amqp091.Connection,
) {
	exists, err := m.exists(ctx, version)
	if err != nil {
		m.logger.Info(fmt.Sprintf("Ошибка при проверке наличия миграции: %v", err))
	}

	if exists {
		m.logger.Info(fmt.Sprintf("Миграция с версией %d уже применена, пропускаем...", version))
		return
	}

	var description string
	for _, conn := range connections {
		ch, err := conn.Channel()
		if err != nil {
			m.logger.Fatal(fmt.Sprintf("Ошибка при создании канала: %v", err))
		}
		description, err = migration(ch)
		if err != nil {
			ch.Close()
			m.logger.Info(fmt.Sprintf("Ошибка при выполнении миграции: %v", err))
		}
		ch.Close()
	}

	if err := m.set(ctx, version, description); err != nil {
		m.logger.Info(fmt.Sprintf("Ошибка при фиксации миграции: %v", err))
	}
}

func (m *Migrations) exists(ctx context.Context, version int) (bool, error) {
	var exists bool

	// Выполняем SQL-запрос для проверки наличия версии
	query := `
        SELECT EXISTS (
            SELECT 1 FROM rabbit_migrations WHERE version_id = $1 AND is_applied = TRUE
        )
    `
	err := m.db.QueryRowContext(ctx, query, version).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}
