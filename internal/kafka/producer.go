package kafka

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"

	"managerFiles/internal/config"
	"managerFiles/internal/model"
)

// Producer публикует события в Kafka топики.
// Используй github.com/segmentio/kafka-go — идиоматичный Go Kafka клиент.
//
// Паттерн: producer асинхронный — WriteMessages возвращает управление сразу,
// сообщения буферизуются и отправляются батчами. Это увеличивает throughput.
type Producer struct {
	writer  *kafka.Writer
	brokers []string
}

// NewProducer создаёт Kafka producer.
func NewProducer(cfg *config.Config) *Producer {
	kafkaWriter := &kafka.Writer{
		Addr:         kafka.TCP(cfg.KafkaBrokers...),
		Balancer:     &kafka.Hash{},          // партиционирование по ключу (UserID)
		BatchSize:    100,                    // отправлять батчами по 100 сообщений
		BatchTimeout: 500 * time.Millisecond, // или каждые 500ms
		RequiredAcks: kafka.RequireAll,       // ждать подтверждения от всех реплик для надёжности
		Async:        true,                   // не блокировать WriteMessages
		MaxAttempts:  3,                      // повторять до 3 раз при ошибках
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		ErrorLogger:  nil,
	}

	return &Producer{
		writer:  kafkaWriter,
		brokers: cfg.KafkaBrokers,
	}
}

// Close сбрасывает буферизованные сообщения и закрывает соединение.
// Вызывается при graceful shutdown.
func (p *Producer) Close() error {
	return p.writer.Close()
}

// PublishFileUploaded публикует событие загрузки файла.
// Ключ партиционирования = UserID — события одного пользователя в одной партиции.
func (p *Producer) PublishAuditLog(ctx context.Context, event *model.EventAuditLog) error {
	return p.publish(ctx, TopicAuditLog, event.UserID, event,
		kafka.Header{Key: "event_id", Value: []byte(event.EventID)},
	)
}

func (p *Producer) PublishFileUploaded(ctx context.Context, event *model.EventFileUploaded) error {
	return p.publish(ctx, TopicFileUploaded, event.UserID, event)
}

func (p *Producer) PublishFileDeleted(ctx context.Context, event *model.EventFileDeleted) error {
	return p.publish(ctx, TopicFileDeleted, event.UserID, event)
}

// publish — внутренний метод сериализации и публикации в любой топик.
func (p *Producer) publish(ctx context.Context, topic, partitionKey string, payload any, headers ...kafka.Header) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	slog.Debug("публикация события в Kafka",
		"topic", topic,
		"key", partitionKey,
		"size", len(data),
	)

	return p.writer.WriteMessages(ctx, kafka.Message{
		Topic:   topic,
		Key:     []byte(partitionKey),
		Value:   data,
		Headers: headers,
	})
}
