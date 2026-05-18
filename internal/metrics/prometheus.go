package metrics

// Prometheus метрики — числовые показатели состояния сервиса.
// Prometheus сервер периодически опрашивает наш /metrics endpoint и сохраняет данные.
// Grafana строит графики по этим данным.
//
// Типы метрик:
// - Counter   — монотонно возрастающий счётчик (запросы, ошибки). Никогда не уменьшается.
// - Gauge     — текущее значение (активные соединения, размер очереди). Может расти и убывать.
// - Histogram — распределение значений (латентность запросов). Строит перцентили (p50, p95, p99).
// - Summary   — похоже на Histogram, но перцентили считаются на клиенте (реже используется).
//
// Зависимость: github.com/prometheus/client_golang/prometheus
//              github.com/prometheus/client_golang/prometheus/promauto
//              github.com/prometheus/client_golang/prometheus/promhttp

import "time"

// Глобальные переменные метрик. Регистрируются один раз в Init().
var (
	// HTTPRequestsTotal — счётчик HTTP запросов с лейблами метода, пути и статус кода.
	// Пример запроса в Prometheus: rate(http_requests_total{status="500"}[5m])
	// HTTPRequestsTotal *prometheus.CounterVec

	// HTTPRequestDuration — гистограмма времени обработки запросов.
	// p95 < 200ms — типичный SLA для API.
	// HTTPRequestDuration *prometheus.HistogramVec

	// ActiveUploads — текущее количество параллельных загрузок файлов.
	// Gauge: Inc() при начале загрузки, Dec() при завершении.
	// ActiveUploads prometheus.Gauge

	// FilesUploadedTotal — счётчик загруженных файлов.
	// FilesUploadedTotal prometheus.Counter

	// FilesUploadedBytes — суммарный объём загруженных данных (байты).
	// FilesUploadedBytes prometheus.Counter

	// WorkerQueueSize — текущий размер очереди воркер пула.
	// WorkerQueueSize prometheus.Gauge

	// KafkaMessagesPublished — счётчик опубликованных Kafka сообщений по топикам.
	// KafkaMessagesPublished *prometheus.CounterVec
)

// Init регистрирует все метрики в prometheus.DefaultRegisterer.
// Вызывается один раз при старте приложения.
func Init() {
	// HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	//     Name: "http_requests_total",
	//     Help: "Общее количество HTTP запросов",
	// }, []string{"method", "path", "status"})

	// HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	//     Name:    "http_request_duration_seconds",
	//     Help:    "Время обработки HTTP запросов",
	//     Buckets: prometheus.DefBuckets, // [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10]
	// }, []string{"method", "path"})

	// ActiveUploads = promauto.NewGauge(prometheus.GaugeOpts{
	//     Name: "active_uploads",
	//     Help: "Текущее количество активных загрузок файлов",
	// })

	// ... остальные метрики
}

// RecordRequest записывает метрики HTTP запроса.
// Вызывается в metrics middleware после обработки каждого запроса.
func RecordRequest(method, path string, status int, duration time.Duration) {
	// HTTPRequestsTotal.WithLabelValues(method, path, strconv.Itoa(status)).Inc()
	// HTTPRequestDuration.WithLabelValues(method, path).Observe(duration.Seconds())
}
