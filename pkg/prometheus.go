package pkg

import "github.com/prometheus/client_golang/prometheus"

type Prometheus struct {
	Http *prometheus.CounterVec
}

func NewPrometheus() *Prometheus {
	return &Prometheus{
		Http: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Количество HTTP-запросов",
			},
			[]string{"path"},
		),
	}
}
