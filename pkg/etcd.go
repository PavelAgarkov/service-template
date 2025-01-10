package pkg

import (
	"context"
	"errors"
	"fmt"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
	"log"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"time"
)

const EtcdClient = "etcd-client-service"

type EtcdClientService struct {
	Client    *clientv3.Client
	leaseMu   sync.Mutex
	lease     *clientv3.LeaseGrantResponse
	logger    *zap.Logger
	serviceId string
	key       string
	host      string
}

func (etcdService *EtcdClientService) GetLease() *clientv3.LeaseGrantResponse {
	etcdService.leaseMu.Lock()
	defer etcdService.leaseMu.Unlock()
	return etcdService.lease
}

func NewEtcdClientService(ctx context.Context, address string, user string, pass string, logger *zap.Logger) (*EtcdClientService, func()) {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{address},
		DialTimeout: 30 * time.Second,
		Username:    user,
		Password:    pass,
	})
	if err != nil {
		log.Fatalf("Не удалось подключиться к ETCD: %v", err)
	}

	etcdService := &EtcdClientService{Client: client, logger: logger}
	lease, err := etcdService.Client.Grant(ctx, 10)
	if err != nil {
		log.Fatalf("Не удалось создать lease: %v", err)
	}

	serviceID := strconv.Itoa(rand.Intn(1000000))

	etcdService.serviceId = serviceID
	etcdService.key = fmt.Sprintf("/services/%s/%s", "my-service", serviceID)
	etcdService.host = fmt.Sprintf("http://%s:8080", etcdService.GetLocalIP())
	etcdService.lease = lease

	return etcdService, etcdService.Close(ctx)
}

func (etcdService *EtcdClientService) Close(ctx context.Context) func() {
	return func() {
		_, err := etcdService.Client.Delete(ctx, etcdService.key)
		if err != nil {
			etcdService.logger.Error(fmt.Sprintf("Не удалось удалить ключ %s: %v", etcdService.key, err))
		} else {
			etcdService.logger.Info(fmt.Sprintf("Ключ %s удалён из etcd", etcdService.key))
		}

		if etcdService.lease != nil {
			_, err = etcdService.Client.Revoke(ctx, etcdService.lease.ID)
			if err != nil {
				etcdService.logger.Error(fmt.Sprintf("Не удалось отозвать lease: %v", err))
			} else {
				etcdService.logger.Info("Lease отозван")
			}
		}

		err = etcdService.Client.Close()
		if err != nil {
			etcdService.logger.Error("Не удалось закрыть соединение с etcd")
		}
		etcdService.logger.Info("Соединение с etcd закрыто")
	}
}

func (etcdService *EtcdClientService) Register(ctx context.Context, logger *zap.Logger, wg *sync.WaitGroup) error {
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer logger.Info("Register goroutine exited") // Для отладки, чтобы видеть, что горутина ушла
		//defer func() {
		//	logger.Info("Register goroutine exited")
		//}()

		for {
			select {
			case <-ctx.Done():
				// как только контекст отменён,
				// выходим из горутины, чтобы не было вечных переподключений
				logger.Info("context done in the Register goroutine, returning...")
				return

			default:
				newService, err := restart(ctx, etcdService)
				if err != nil {
					if errors.Is(err, closedErr) {
						logger.Info("KeepAlive channel closed, reconnecting...")
						continue
					}
					if errors.Is(err, contextClosedErr) {
						etcdService.logger.Info("context error canceled 2")
						return
					}

					// Любая другая ошибка — логируем и завершаем горутину, чтобы не зациклиться
					etcdService.logger.Warn("unexpected error while restart", zap.Error(err))
					return
				}

				// Если newService == nil, значит переподключение не удалось полностью
				// Можно выйти или повторять бесконечно — зависит от логики
				if newService == nil {
					logger.Info("Переподключение не удалось (newService == nil), выходим из цикла")
					return
				}

				etcdService = newService
			}
		}
	}()

	return nil
}

var closedErr = errors.New("closerd")
var contextClosedErr = errors.New("context closed")

func restart(ctx context.Context, etcdService *EtcdClientService) (*EtcdClientService, error) {
	log.Println("KeepAlive канал закрыт")
	reconnectAttempts := 0
	maxReconnectAttempts := 5

	for reconnectAttempts < maxReconnectAttempts {
		select {
		case <-ctx.Done():
			etcdService.logger.Info("context error canceled 1")
			return nil, contextClosedErr
		default:
			// продолжаем
		}

		reconnectAttempts++
		log.Printf("Попытка подключения %d/%d...\n", reconnectAttempts, maxReconnectAttempts)

		newClient, err := clientv3.New(clientv3.Config{
			Endpoints:   []string{"http://localhost:2379"}, // Замените address на ваши настройки
			DialTimeout: 2 * time.Second,
			Username:    "admin",
			Password:    "adminpassword",
		})
		if err != nil {
			log.Printf("Ошибка переподключения: %v\n", err)
			// Немного ждём и повторяем
			select {
			case <-ctx.Done():
				etcdService.logger.Info("context error canceled 1")
				return nil, contextClosedErr
			case <-time.After(1 * time.Second):
			}
			continue
		}

		log.Println("Соединение с etcd восстановлено. Перезапускаем lease...")

		lease, err := newClient.Grant(ctx, 10)
		if err != nil {
			newClient.Close()
			log.Printf("Ошибка при восстановлении lease: %v\n", err)
			continue
		}

		newService := &EtcdClientService{
			Client:    newClient,
			lease:     lease,
			logger:    etcdService.logger,
			serviceId: etcdService.serviceId,
			key:       etcdService.key,
			host:      etcdService.host,
		}

		_, err = newClient.Put(ctx, newService.key, newService.host, clientv3.WithLease(lease.ID))
		if err != nil {
			log.Printf("Ошибка при повторной регистрации ключа: %v\n", err)
			newClient.Close()
			continue
		}
		log.Println("Ключ успешно зарегистрирован заново.")

		// Перезапускаем KeepAlive
		ch, kaErr := newClient.KeepAlive(ctx, lease.ID)
		if kaErr != nil {
			log.Printf("Ошибка настройки KeepAlive: %v\n", kaErr)
			newClient.Close()
			continue
		}

		log.Println("KeepAlive перезапущен.")

		// Здесь блокирующая часть: ждём keepalive-сообщения в цикле
		for {
			select {
			case ka, ok := <-ch:
				if !ok {
					log.Println("KeepAlive канал закрыт")
					log.Println("KeepAlive канал закрыт, попытка переподключения...")
					return newService, closedErr
				}
				log.Printf("Получено KeepAlive сообщение: TTL=%d\n", ka.TTL)

			case <-ctx.Done():
				log.Println("context done")
				return newService, ctx.Err()
			}
		}
	}

	// Если за maxReconnectAttempts не смогли переподключиться,
	// возвращаем nil, чтобы показать «невозможно восстановить».
	return etcdService, nil
}

func (etcdService *EtcdClientService) GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Fatalf("Не удалось получить IP адрес: %v", err)
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			return ipnet.IP.String()
		}
	}
	return "127.0.0.1"
}
