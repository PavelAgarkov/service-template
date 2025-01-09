package pkg

import (
	"context"
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

func NewEtcdClientService(address string, user string, pass string, logger *zap.Logger) (*EtcdClientService, func()) {
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
	lease, err := etcdService.Client.Grant(context.Background(), 10)
	if err != nil {
		log.Fatalf("Не удалось создать lease: %v", err)
	}

	serviceID := strconv.Itoa(rand.Intn(1000000))

	etcdService.serviceId = serviceID
	etcdService.key = fmt.Sprintf("/services/%s/%s", "my-service", serviceID)
	etcdService.host = fmt.Sprintf("http://%s:8080", etcdService.GetLocalIP())
	etcdService.lease = lease

	return etcdService, etcdService.Close
}

func (etcdService *EtcdClientService) Close() {
	_, err := etcdService.Client.Delete(context.Background(), etcdService.key)
	if err != nil {
		etcdService.logger.Error(fmt.Sprintf("Не удалось удалить ключ %s: %v", etcdService.key, err))
	} else {
		etcdService.logger.Info(fmt.Sprintf("Ключ %s удалён из etcd", etcdService.key))
	}

	if etcdService.lease != nil {
		_, err = etcdService.Client.Revoke(context.Background(), etcdService.lease.ID)
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

func (etcdService *EtcdClientService) Register(ctx context.Context, logger *zap.Logger) error {
	_, err := etcdService.Client.Put(context.Background(), etcdService.key, etcdService.host, clientv3.WithLease(etcdService.lease.ID))
	if err != nil {
		logger.Fatal("Не удалось зарегистрировать сервис: %v", zap.Error(err))
	}
	log.Printf("Сервис зарегистрирован с ключом: %s и значением: %s", etcdService.key, etcdService.host)

	ch, kaerr := etcdService.Client.KeepAlive(context.Background(), etcdService.lease.ID)
	if kaerr != nil {
		log.Fatalf("Не удалось настроить KeepAlive: %v", kaerr)
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Println("recovered KeepAlive канал закрыт")
			}
		}()
		for {
			select {
			case ka, ok := <-ch:
				if !ok {
					log.Println("KeepAlive канал закрыт")
					return
				}
				log.Printf("Получено KeepAlive сообщение: TTL=%d", ka.TTL)
			case <-ctx.Done():
				logger.Info("context done")
				return
			}
		}
	}()

	return nil
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
