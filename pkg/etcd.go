package pkg

import (
	"context"
	"errors"
	"fmt"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"go.uber.org/zap"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"time"
)

const EtcdClient = "etcd-client-service"

type EtcdClientService struct {
	Client       *clientv3.Client
	leaseMu      sync.Mutex
	lease        *clientv3.LeaseGrantResponse
	logger       *zap.Logger
	register     *sync.WaitGroup
	session      *concurrency.Session
	serviceId    string
	host         string
	key          string
	ShutdownFunc func()
}

func (etcdService *EtcdClientService) GetLease() *clientv3.LeaseGrantResponse {
	etcdService.leaseMu.Lock()
	defer etcdService.leaseMu.Unlock()
	return etcdService.lease
}

func NewEtcdClientService(
	ctx context.Context,
	address string,
	user string,
	pass string,
	port string,
	protocolPrefix string,
	key string,
	serviceId string,
	logger *zap.Logger,
) *EtcdClientService {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{address},
		DialTimeout: 5 * time.Second,
		Username:    user,
		Password:    pass,
	})
	if err != nil {
		logger.Fatal("Не удалось подключиться к ETCD: %v", zap.Error(err))
	}

	etcdService := &EtcdClientService{Client: client, logger: logger, register: &sync.WaitGroup{}}
	lease, err := etcdService.Client.Grant(ctx, 10)
	if err != nil {
		logger.Fatal("Не удалось создать lease: %v", zap.Error(err))
	}

	etcdService.serviceId = serviceId
	etcdService.key = key
	etcdService.host = fmt.Sprintf("%s://%s%s", protocolPrefix, etcdService.GetLocalIP(), port)
	etcdService.lease = lease
	etcdService.ShutdownFunc = etcdService.Close(ctx)

	return etcdService
}

func (etcdService *EtcdClientService) Close(ctx context.Context) func() {
	return func() {
		//_, err := etcdService.Client.Delete(ctx, etcdService.Key)
		//if err != nil {
		//	etcdService.logger.Error(fmt.Sprintf("Не удалось удалить ключ %s: %v", etcdService.Key, err))
		//} else {
		//	etcdService.logger.Info(fmt.Sprintf("Ключ %s удалён из etcd", etcdService.Key))
		//}

		var err error
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
		etcdService.register.Wait()
	}
}

func (etcdService *EtcdClientService) Register(ctx context.Context, logger *zap.Logger) error {
	etcdService.register.Add(1)
	go func() {
		defer etcdService.register.Done()
		defer logger.Info("Register goroutine exited") // Для отладки, чтобы видеть, что горутина ушла
		defer func() {
			if r := recover(); r != nil {
				logger.Error("Recovered from panic", zap.Any("panic", r))
			}
		}()

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
					if errors.Is(err, contextCancelledErr) {
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

var closedErr = errors.New("closer error")
var contextCancelledErr = errors.New("context canceled")

func restart(ctx context.Context, etcdService *EtcdClientService) (*EtcdClientService, error) {
	etcdService.logger.Info("KeepAlive канал закрыт")
	reconnectAttempts := 0
	maxReconnectAttempts := 5

	for reconnectAttempts < maxReconnectAttempts {
		select {
		case <-ctx.Done():
			etcdService.logger.Info("context error canceled 1")
			return nil, contextCancelledErr
		default:
			// продолжаем
		}

		reconnectAttempts++
		etcdService.logger.Info(fmt.Sprintf("Попытка подключения %d/%d...\n", reconnectAttempts, maxReconnectAttempts))

		newClient, err := clientv3.New(clientv3.Config{
			Endpoints:   []string{"http://localhost:2379"}, // Замените address на ваши настройки
			DialTimeout: 2 * time.Second,
			Username:    "admin",
			Password:    "adminpassword",
		})
		if err != nil {
			etcdService.logger.Info(fmt.Sprintf("Ошибка переподключения: %v\n", err))
			// Немного ждём и повторяем
			select {
			case <-ctx.Done():
				etcdService.logger.Info("context error canceled 1")
				return nil, contextCancelledErr
			case <-time.After(1 * time.Second):
			}
			continue
		}

		etcdService.logger.Info("Соединение с etcd восстановлено. Перезапускаем lease...")

		lease, err := newClient.Grant(ctx, 10)
		if err != nil {
			newClient.Close()
			etcdService.logger.Info(fmt.Sprintf("Ошибка при восстановлении lease: %v\n", err))
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
			etcdService.logger.Info(fmt.Sprintf("Ошибка при повторной регистрации ключа: %v\n", err))
			newClient.Close()
			continue
		}
		etcdService.logger.Info("Ключ успешно зарегистрирован заново.")

		// Перезапускаем KeepAlive
		ch, kaErr := newClient.KeepAlive(ctx, lease.ID)
		if kaErr != nil {
			etcdService.logger.Info(fmt.Sprintf("Ошибка настройки KeepAlive: %v\n", kaErr))
			newClient.Close()
			continue
		}

		etcdService.logger.Info("KeepAlive перезапущен.")

		// Здесь блокирующая часть: ждём keepalive-сообщения в цикле
		for {
			select {
			case ka, ok := <-ch:
				if !ok {
					etcdService.logger.Info("KeepAlive канал закрыт")
					etcdService.logger.Info("KeepAlive канал закрыт, попытка переподключения...")
					return newService, contextCancelledErr
				}
				etcdService.logger.Info(fmt.Sprintf("Получено KeepAlive сообщение: TTL=%d\n", ka.TTL))

			case <-ctx.Done():
				newService.logger.Info("context done")
				return newService, contextCancelledErr
			}
		}
	}

	// Если за maxReconnectAttempts не смогли переподключиться,
	// возвращаем nil, чтобы показать «невозможно восстановить».
	return etcdService, nil
}

func (etcdService *EtcdClientService) CreateSession(opts ...concurrency.SessionOption) (*concurrency.Session, error) {
	if etcdService.session != nil {
		return etcdService.session, nil
	}
	etcdService.session, _ = concurrency.NewSession(etcdService.Client, opts...)
	return etcdService.session, nil
}

func (etcdService *EtcdClientService) StopSession() {
	if etcdService.session != nil {
		err := etcdService.session.Close()
		if err != nil {
			etcdService.logger.Error("Не удалось закрыть сессию", zap.Error(err))
		}
		etcdService.session = nil
	}
}

func (etcdService *EtcdClientService) GetSession() *concurrency.Session {
	return etcdService.session
}

func (etcdService *EtcdClientService) GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		etcdService.logger.Fatal("Не удалось получить IP адрес: %v", zap.Error(err))
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			return ipnet.IP.String()
		}
	}
	return "127.0.0.1"
}

func NewServiceId() string {
	return strconv.Itoa(rand.Intn(1000000))
}

func NewServiceKey(serviceId string, sName string) string {
	return fmt.Sprintf("/services/%s/%s", sName, serviceId)
}
