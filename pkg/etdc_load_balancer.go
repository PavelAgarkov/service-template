package pkg

import (
	"context"
	"github.com/oklog/run"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"
)

// Backend представляет один backend-сервер (API-под)
type Backend struct {
	URL          *url.URL
	Alive        bool
	ReverseProxy *httputil.ReverseProxy
	Mutex        sync.RWMutex
}

// SetAlive устанавливает статус доступности backend-сервера
func (b *Backend) SetAlive(alive bool) {
	b.Mutex.Lock()
	b.Alive = alive
	b.Mutex.Unlock()
}

// IsAlive возвращает статус доступности backend-сервера
func (b *Backend) IsAlive() bool {
	b.Mutex.RLock()
	defer b.Mutex.RUnlock()
	return b.Alive
}

// LoadBalancer управляет списком backend-серверов и балансировкой нагрузки
type LoadBalancer struct {
	Backends []*Backend
	Current  int
	Mutex    sync.Mutex
	Logger   *zap.Logger
	service  *EtcdClientService
}

func NewLoadBalancer(logger *zap.Logger, service *EtcdClientService) *LoadBalancer {
	return &LoadBalancer{
		Backends: []*Backend{},
		Current:  0,
		Logger:   logger,
		service:  service,
	}
}

func (lb *LoadBalancer) Run(ctx context.Context, watchPrefix string, g *run.Group) {
	lb.UpdateBackends(ctx, lb.service.Client, watchPrefix)

	stopWatch := make(chan struct{})
	g.Add(
		func() error {
			lb.WatchBackends(ctx, lb.service.Client, watchPrefix, stopWatch)
			return nil
		},
		func(err error) {
			close(stopWatch)
			lb.Logger.Info("Завершение работы WatchBackends")
		})

	stopHealthcheck := make(chan struct{})

	g.Add(
		func() error {
			lb.HealthCheck(ctx, 10*time.Second, stopHealthcheck)
			return nil
		},
		func(err error) {
			close(stopHealthcheck)
			lb.Logger.Info("Завершение работы HealthCheck")
		})
}

func (lb *LoadBalancer) NextBackend() *Backend {
	lb.Mutex.Lock()
	defer lb.Mutex.Unlock()
	if len(lb.Backends) == 0 {
		return nil
	}
	backend := lb.Backends[lb.Current]
	lb.Current = (lb.Current + 1) % len(lb.Backends)
	if backend.IsAlive() {
		return backend
	}
	// Если текущий backend не доступен, ищем следующий
	for i := 0; i < len(lb.Backends); i++ {
		backend = lb.Backends[lb.Current]
		lb.Current = (lb.Current + 1) % len(lb.Backends)
		if backend.IsAlive() {
			return backend
		}
	}
	return nil
}

// ServeHTTP реализует интерфейс http.Handler для балансировщика нагрузки
func (lb *LoadBalancer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	backend := lb.NextBackend()
	if backend != nil {
		lb.Logger.Info("Направление запроса", zap.String("backend", backend.URL.String()))
		backend.ReverseProxy.ServeHTTP(w, r)
	} else {
		lb.Logger.Warn("Нет доступных backend-серверов")
		http.Error(w, "Service not available", http.StatusServiceUnavailable)
	}
}

func (lb *LoadBalancer) UpdateBackends(ctx context.Context, client *clientv3.Client, servicePrefix string) {
	resp, err := client.Get(ctx, servicePrefix, clientv3.WithPrefix())
	if err != nil {
		lb.Logger.Error("Ошибка при получении backend-серверов из etcd", zap.Error(err))
		return
	}

	var newBackends []*Backend
	for _, kv := range resp.Kvs {
		key := string(kv.Key)
		value := string(kv.Value)

		// Игнорируем собственный сервис
		if key == lb.service.Key {
			continue
		}

		backendURL, err := url.Parse(value)
		if err != nil {
			lb.Logger.Error("Неверный URL backend-сервера", zap.String("url", value), zap.Error(err))
			continue
		}

		// Проверяем, уже ли этот backend существует
		exists := false
		for _, b := range lb.Backends {
			if b.URL.String() == backendURL.String() {
				newBackends = append(newBackends, b)
				exists = true
				break
			}
		}

		if !exists {
			// Добавляем новый backend
			newBackends = append(newBackends, &Backend{
				URL:          backendURL,
				Alive:        true, // Предполагаем, что сервер живой при добавлении
				ReverseProxy: httputil.NewSingleHostReverseProxy(backendURL),
			})
			lb.Logger.Info("Добавлен новый backend-сервер", zap.String("url", backendURL.String()))
		}
	}

	// Обновляем список backend-серверов
	lb.Mutex.Lock()
	defer lb.Mutex.Unlock()
	lb.Backends = newBackends
	lb.Current = 0

	lb.Logger.Info("Список backend-серверов обновлён", zap.Int("count", len(newBackends)))
	for k, v := range lb.Backends {
		lb.Logger.Info(v.URL.String(), zap.Int("index", k))
	}
}

func (lb *LoadBalancer) WatchBackends(
	ctx context.Context,
	client *clientv3.Client,
	servicePrefix string,
	stop chan struct{},
) {
	defer func() {
		if r := recover(); r != nil {
			lb.Logger.Error("Паника в WatchBackends", zap.Any("recover", r))
		}
	}()

	watchChan := client.Watch(ctx, servicePrefix, clientv3.WithPrefix())
Watch:
	for {
		select {
		case <-stop:
			lb.Logger.Info("WatchBackends остановлена")
			return
		case <-ctx.Done():
			lb.Logger.Info("WatchBackends завершена")
			return
		case watchResp, ok := <-watchChan:
			if !ok {
				lb.Logger.Error("Ошибка при получении событий из etcd")
				break Watch
			}
			for _, ev := range watchResp.Events {
				switch ev.Type {
				case clientv3.EventTypePut:
					lb.Logger.Info("Получено событие PUT", zap.String("key", string(ev.Kv.Key)), zap.String("value", string(ev.Kv.Value)))
				case clientv3.EventTypeDelete:
					lb.Logger.Info("Получено событие DELETE", zap.String("key", string(ev.Kv.Key)))
				}
			}
			lb.UpdateBackends(ctx, client, servicePrefix)
		}
	}
	lb.Logger.Info("Канал WatchBackends закрыт")
}

// HealthCheck периодически проверяет доступность backend-серверов
func (lb *LoadBalancer) HealthCheck(ctx context.Context, interval time.Duration, stop chan struct{}) {
	defer func() {
		if r := recover(); r != nil {
			lb.Logger.Error("Паника в HealthCheck", zap.Any("recover", r))
		}
	}()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			lb.Logger.Info("HealthCheck остановлена")
			return
		case <-ticker.C:
			lb.Logger.Info("Запуск проверки здоровья backend-серверов")
			lb.Mutex.Lock()
			for _, backend := range lb.Backends {
				go func(b *Backend) {
					alive := isBackendAlive(b.URL)
					b.SetAlive(alive)
					status := "up"
					if !alive {
						status = "down"
					}
					lb.Logger.Info("Проверка здоровья", zap.String("backend", b.URL.String()), zap.String("status", status))
				}(backend)
			}
			lb.Mutex.Unlock()
		case <-ctx.Done():
			lb.Logger.Info("HealthCheck завершена")
			return
		}
	}
}

// isBackendAlive проверяет, доступен ли backend-сервер
func isBackendAlive(u *url.URL) bool {
	timeout := 2 * time.Second
	client := http.Client{
		Timeout: timeout,
	}

	// Предполагается, что у backend-сервера есть эндпоинт /health
	healthURL := *u
	healthURL.Path = "/health"

	resp, err := client.Get(healthURL.String())
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}
