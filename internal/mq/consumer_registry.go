package mq

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"

	"github.com/krc/rag/internal/mq/kafka"
)

// ConsumerHandler processes a consumed message.
type ConsumerHandler func(ctx context.Context, key string, value []byte) error

// ConsumerConfig holds consumer configuration.
type ConsumerConfig struct {
	Brokers    []string      // Kafka broker addresses
	Topic      string        // Topic to consume from
	GroupID    string        // Consumer group ID
	Handler    ConsumerHandler // Message handler function
	AutoCommit bool          // Whether to auto-commit offsets
}

// ConsumerRegistry manages multiple Kafka consumers and their lifecycle.
type ConsumerRegistry struct {
	consumers      []ConsumerConfig
	kafkaConsumers []*kafka.Consumer
	logger         *zap.Logger
	mu             sync.Mutex
	running        bool
	ctx            context.Context
	cancel         context.CancelFunc
}

// NewConsumerRegistry creates a new consumer registry.
func NewConsumerRegistry(logger *zap.Logger) *ConsumerRegistry {
	return &ConsumerRegistry{
		consumers:      make([]ConsumerConfig, 0),
		kafkaConsumers: make([]*kafka.Consumer, 0),
		logger:         logger,
	}
}

// Register registers a consumer configuration.
func (r *ConsumerRegistry) Register(cfg ConsumerConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.consumers = append(r.consumers, cfg)
}

// Start starts all registered consumers.
func (r *ConsumerRegistry) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.running {
		return fmt.Errorf("consumer registry is already running")
	}

	r.ctx, r.cancel = context.WithCancel(ctx)
	r.logger.Info("starting consumer registry", zap.Int("consumers", len(r.consumers)))

	// Create and start Kafka consumers
	for _, cfg := range r.consumers {
		if len(cfg.Brokers) == 0 {
			r.logger.Warn("skipping consumer with no brokers", zap.String("topic", cfg.Topic))
			continue
		}

		kafkaConsumer, err := kafka.NewConsumer(kafka.ConsumerConfig{
			Brokers:    cfg.Brokers,
			Topic:      cfg.Topic,
			GroupID:    cfg.GroupID,
			Handler:    cfg.Handler,
			AutoCommit: cfg.AutoCommit,
			Logger:     r.logger,
		})
		if err != nil {
			r.logger.Error("failed to create kafka consumer",
				zap.String("topic", cfg.Topic),
				zap.Error(err),
			)
			continue
		}

		if err := kafkaConsumer.Start(r.ctx); err != nil {
			r.logger.Error("failed to start kafka consumer",
				zap.String("topic", cfg.Topic),
				zap.Error(err),
			)
			continue
		}

		r.kafkaConsumers = append(r.kafkaConsumers, kafkaConsumer)
	}

	r.running = true
	r.logger.Info("consumer registry started", zap.Int("consumers", len(r.kafkaConsumers)))

	return nil
}

// Stop stops all registered consumers.
func (r *ConsumerRegistry) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.running {
		return
	}

	r.logger.Info("stopping consumer registry", zap.Int("consumers", len(r.kafkaConsumers)))

	// Cancel context to signal all consumers to stop
	if r.cancel != nil {
		r.cancel()
	}

	// Stop all Kafka consumers
	for _, consumer := range r.kafkaConsumers {
		if err := consumer.Stop(); err != nil {
			r.logger.Error("failed to stop kafka consumer", zap.Error(err))
		}
	}

	r.kafkaConsumers = nil
	r.running = false
	r.logger.Info("consumer registry stopped")
}

// Close closes the consumer registry (implements bootstrap.Resource interface).
func (r *ConsumerRegistry) Close() error {
	r.Stop()
	return nil
}


