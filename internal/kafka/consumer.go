package kafka

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	"managerFiles/internal/config"
	"managerFiles/internal/model"
)

// Consumer читает сообщения из одного Kafka топика в бесконечном цикле.
// Каждый Consumer запускается в отдельной горутине.
// Consumer Group: несколько инстансов сервиса читают разные партиции одного топика
// (горизонтальное масштабирование обработки).
type Consumer struct {
	// reader *kafka.Reader (из kafka-go)
	topic  string
	closed chan struct{}
}

// StartConsumers запускает всех consumers в горутинах.
// Возвращает слайс consumers для последующего Close при shutdown.
// wg используется для ожидания завершения всех горутин.
func StartConsumers(ctx context.Context, cfg *config.Config, wg *sync.WaitGroup) []*Consumer {
	consumers := []*Consumer{
		newConsumer(cfg, TopicFileDeleted),
		newConsumer(cfg, TopicAuditLog),
	}

	for _, c := range consumers {
		wg.Add(1)
		go func(consumer *Consumer) {
			defer wg.Done()
			consumer.Run(ctx)
		}(c)
	}

	return consumers
}

// newConsumer создаёт reader для топика.
func newConsumer(cfg *config.Config, topic string) *Consumer {
	// kafka.NewReader(kafka.ReaderConfig{
	//     Brokers:  cfg.KafkaBrokers,
	//     GroupID:  cfg.KafkaGroupID,  // consumer group для load balancing между инстансами
	//     Topic:    topic,
	//     MinBytes: 1,                 // минимальный размер fetch (1 байт — без ожидания)
	//     MaxBytes: 10e6,              // максимальный fetch (10 MB)
	// })
	return &Consumer{topic: topic, closed: make(chan struct{})}
}

// Run запускает бесконечный цикл чтения сообщений.
// Завершается когда ctx отменён или вызван Close.
func (c *Consumer) Run(ctx context.Context) {
	slog.Info("consumer запущен", "topic", c.topic)

	for {
		select {
		case <-ctx.Done():
			slog.Info("consumer останавливается", "topic", c.topic)
			return
		default:
		}

		// msg, err := c.reader.ReadMessage(ctx)
		// if err != nil {
		//     if errors.Is(err, context.Canceled) { return }
		//     slog.Error("ошибка чтения из Kafka", "topic", c.topic, "err", err)
		//     continue
		// }
		//
		// Обработать сообщение в зависимости от топика.
		// При ошибке обработки — логировать и продолжать (не останавливать consumer).
		// Для критических ошибок можно отправить в dead-letter queue (отдельный топик).
		//
		// После успешной обработки Kafka-go автоматически коммитит offset в group.

		if err := c.handleByTopic(ctx, c.topic, nil); err != nil {
			slog.Error("ошибка обработки сообщения", "topic", c.topic, "err", err)
		}

		return // убрать когда подключим реальный reader
	}
}

// Close останавливает consumer.
func (c *Consumer) Close() error {
	// c.reader.Close()
	return nil
}

// handleByTopic маршрутизирует сообщение на нужный обработчик.
func (c *Consumer) handleByTopic(ctx context.Context, topic string, rawMsg []byte) error {
	switch topic {
	case TopicFileDeleted:
		return c.handleFileDeleted(ctx, rawMsg)
	case TopicAuditLog:
		return c.handleAuditLog(ctx, rawMsg)
	}
	return nil
}

// handleFileDeleted удаляет физический объект из MinIO.
// Вызывается когда файл был soft-deleted в PostgreSQL.
func (c *Consumer) handleFileDeleted(ctx context.Context, rawMsg []byte) error {
	var event model.EventFileDeleted
	if err := json.Unmarshal(rawMsg, &event); err != nil {
		return err
	}

	slog.Info("обработка удаления файла", "file_id", event.FileID, "key", event.StorageKey)

	// 1. Получить MinIO client (нужно внедрить зависимость в consumer)
	// 2. storage.Delete(ctx, event.StorageKey)
	// 3. Логировать результат
	return nil
}

// handleAuditLog сохраняет событие аудита в PostgreSQL.
func (c *Consumer) handleAuditLog(ctx context.Context, rawMsg []byte) error {
	var event model.EventAuditLog
	if err := json.Unmarshal(rawMsg, &event); err != nil {
		return err
	}

	slog.Info("аудит лог", "user", event.UserID, "action", event.Action, "success", event.Success)

	// 1. Преобразовать EventAuditLog -> model.AuditLog
	// 2. auditRepo.Create(ctx, &auditLog)
	return nil
}
