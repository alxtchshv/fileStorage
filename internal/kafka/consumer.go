package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"

	"managerFiles/internal/config"
	"managerFiles/internal/model"

	"github.com/segmentio/kafka-go"
)

// Consumer читает сообщения из одного Kafka топика в бесконечном цикле.
// Consumer Group: несколько инстансов сервиса читают разные партиции одного топика.
type Consumer struct {
	reader *kafka.Reader
	topic  string
}

// StartConsumers запускает всех consumers в горутинах.
// wg нужен для graceful shutdown — ждём завершения горутин перед остановкой.
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

func newConsumer(cfg *config.Config, topic string) *Consumer {
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:  cfg.KafkaBrokers,
			GroupID:  cfg.KafkaGroupID,
			Topic:    topic,
			MinBytes: 1,
			MaxBytes: 10e6,
		}),
		topic: topic,
	}
}

// Run читает сообщения пока ctx не отменён.
// При ошибке чтения — логирует и продолжает (не останавливает consumer).
func (c *Consumer) Run(ctx context.Context) {
	slog.Info("consumer запущен", "topic", c.topic)
	for {
		msg, err := c.reader.ReadMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				slog.Info("consumer остановлен", "topic", c.topic)
				return
			}
			slog.Error("ошибка чтения из Kafka", "topic", c.topic, "err", err)
			continue
		}
		if err := c.handleByTopic(ctx, msg.Value); err != nil {
			slog.Error("ошибка обработки сообщения", "topic", c.topic, "err", err)
		}
	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}

func (c *Consumer) handleByTopic(ctx context.Context, rawMsg []byte) error {
	switch c.topic {
	case TopicFileDeleted:
		return c.handleFileDeleted(ctx, rawMsg)
	case TopicAuditLog:
		return c.handleAuditLog(ctx, rawMsg)
	}
	return nil
}

// handleFileDeleted удаляет физический объект из MinIO.
func (c *Consumer) handleFileDeleted(ctx context.Context, rawMsg []byte) error {
	var event model.EventFileDeleted
	if err := json.Unmarshal(rawMsg, &event); err != nil {
		return err
	}
	slog.Info("удаление файла из хранилища", "file_id", event.FileID, "key", event.StorageKey)
	// storage.Delete(ctx, event.StorageKey) — нужно внедрить зависимость в consumer
	return nil
}

// handleAuditLog сохраняет событие аудита в PostgreSQL.
func (c *Consumer) handleAuditLog(ctx context.Context, rawMsg []byte) error {
	var event model.EventAuditLog
	if err := json.Unmarshal(rawMsg, &event); err != nil {
		return err
	}
	slog.Info("audit", "user", event.UserID, "action", event.Action, "ok", event.Success)
	// auditRepo.Create(ctx, &model.AuditLog{...})
	return nil
}
