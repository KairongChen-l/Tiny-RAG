package messaging

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
)

// KafkaProducer wraps Kafka producer for sending messages.
type KafkaProducer struct {
	producer sarama.SyncProducer
	logger   *zap.Logger
}

// ProducerConfig holds Kafka producer configuration.
type ProducerConfig struct {
	Brokers []string
	Logger  *zap.Logger
}

// NewKafkaProducer creates a new Kafka producer.
func NewKafkaProducer(cfg ProducerConfig) (*KafkaProducer, error) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5

	producer, err := sarama.NewSyncProducer(cfg.Brokers, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka producer: %w", err)
	}

	return &KafkaProducer{
		producer: producer,
		logger:   cfg.Logger,
	}, nil
}

// SendMessage sends a message to a Kafka topic.
func (p *KafkaProducer) SendMessage(ctx context.Context, topic string, key string, value interface{}) error {
	var valueBytes []byte
	var err error

	switch v := value.(type) {
	case []byte:
		valueBytes = v
	case string:
		valueBytes = []byte(v)
	default:
		valueBytes, err = json.Marshal(value)
		if err != nil {
			return fmt.Errorf("failed to marshal message: %w", err)
		}
	}

	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(key),
		Value: sarama.ByteEncoder(valueBytes),
	}

	partition, offset, err := p.producer.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	p.logger.Debug("message sent",
		zap.String("topic", topic),
		zap.Int32("partition", partition),
		zap.Int64("offset", offset),
	)

	return nil
}

// Close closes the producer.
func (p *KafkaProducer) Close() error {
	return p.producer.Close()
}
