package kafka

import (
	"context"
	"encoding/json"
	"log/slog"

	"managerFiles/internal/config"
	"managerFiles/internal/model"
)

// Producer публикует события в Kafka топики.
// Используй github.com/segmentio/kafka-go — идиоматичный Go Kafka клиент.
//
// Паттерн: producer асинхронный — WriteMessages возвращает управление сразу,
// сообщения буферизуются и отправляются батчами. Это увеличивает throughput.
type Producer struct {
	// writer *kafka.Writer (из kafka-go)
	brokers []string
}

// NewProducer создаёт Kafka producer.
func NewProducer(cfg *config.Config) *Producer {
	// kafka.NewWriter(&kafka.WriterConfig{
	//     Brokers:      cfg.KafkaBrokers,
	//     Balancer:     &kafka.Hash{},     // партиционирование по ключу (user_id)
	//     RequiredAcks: kafka.RequireAll,  // ждать подтверждения от всех реплик (надёжность)
	//     Async:        false,             // синхронная запись для надёжности
	//     Compression:  kafka.Snappy,      // сжатие для экономии трафика
	// })
	return &Producer{brokers: cfg.KafkaBrokers}
}

// Close сбрасывает буферизованные сообщения и закрывает соединение.
// Вызывается при graceful shutdown.
func (p *Producer) Close() error {
	// return p.writer.Close()
	return nil
}

// PublishFileUploaded публикует событие загрузки файла.
// Ключ партиционирования = UserID — события одного пользователя в одной партиции.
func (p *Producer) PublishFileUploaded(ctx context.Context, event *model.EventFileUploaded) error {
	// 1. Сериализовать event в JSON
	// 2. Опубликовать в TopicFileUploaded с ключом event.UserID
	// kafka.Message{
	//     Topic: TopicFileUploaded,
	//     Key:   []byte(event.UserID),
	//     Value: jsonBytes,
	//     Headers: []kafka.Header{{"event_id", []byte(event.EventID)}},
	// }
	return p.publish(ctx, TopicFileUploaded, event.UserID, event)
}

// PublishFileDeleted публикует событие удаления файла.
func (p *Producer) PublishFileDeleted(ctx context.Context, event *model.EventFileDeleted) error {
	return p.publish(ctx, TopicFileDeleted, event.UserID, event)
}

// PublishAuditLog публикует событие аудита.
func (p *Producer) PublishAuditLog(ctx context.Context, event *model.EventAuditLog) error {
	return p.publish(ctx, TopicAuditLog, event.UserID, event)
}

// publish — внутренний метод сериализации и публикации в любой топик.
func (p *Producer) publish(ctx context.Context, topic, partitionKey string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	slog.Debug("публикация события в Kafka", "topic", topic, "key", partitionKey, "size", len(data))

	// p.writer.WriteMessages(ctx, kafka.Message{
	//     Topic: topic,
	//     Key:   []byte(partitionKey),
	//     Value: data,
	// })
	_ = data
	return nil
}
