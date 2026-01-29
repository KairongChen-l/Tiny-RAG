// Package kafka provides Kafka consumer implementation using segmentio/kafka-go.
package kafka

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// Consumer represents a Kafka consumer that can consume messages from a topic.
type Consumer struct {
	reader      *kafka.Reader
	handler     func(ctx context.Context, key string, value []byte) error
	logger      *zap.Logger
	autoCommit  bool
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	mu          sync.Mutex
	running     bool
}

// ConsumerConfig holds configuration for a Kafka consumer.
type ConsumerConfig struct {
	Brokers    []string
	Topic      string
	GroupID    string
	Handler    func(ctx context.Context, key string, value []byte) error
	AutoCommit bool
	Logger     *zap.Logger
}

// NewConsumer creates a new Kafka consumer.
func NewConsumer(cfg ConsumerConfig) (*Consumer, error) {
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("brokers are required")
	}
	if cfg.Topic == "" {
		return nil, fmt.Errorf("topic is required")
	}
	if cfg.GroupID == "" {
		return nil, fmt.Errorf("group_id is required")
	}
	if cfg.Handler == nil {
		return nil, fmt.Errorf("handler is required")
	}
	if cfg.Logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	readerConfig := kafka.ReaderConfig{
		Brokers:  cfg.Brokers,
		Topic:    cfg.Topic,
		GroupID:  cfg.GroupID,
		MinBytes: 10e3, // 10KB
		MaxBytes: 10e6, // 10MB
		MaxWait:  1 * time.Second,
	}

	reader := kafka.NewReader(readerConfig)

	ctx, cancel := context.WithCancel(context.Background())

	return &Consumer{
		reader:     reader,
		handler:    cfg.Handler,
		logger:     cfg.Logger,
		autoCommit: cfg.AutoCommit,
		ctx:        ctx,
		cancel:     cancel,
	}, nil
}

// Start starts consuming messages.
func (c *Consumer) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return fmt.Errorf("consumer is already running")
	}

	c.running = true
	c.wg.Add(1)

	go c.consume(ctx)

	c.logger.Info("kafka consumer started",
		zap.String("topic", c.reader.Config().Topic),
		zap.String("group_id", c.reader.Config().GroupID),
		zap.Bool("auto_commit", c.autoCommit),
	)

	return nil
}

// consume reads messages from Kafka and processes them.
func (c *Consumer) consume(ctx context.Context) {
	defer c.wg.Done()

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("kafka consumer context cancelled, stopping")
			return
		case <-c.ctx.Done():
			c.logger.Info("kafka consumer stopped")
			return
		default:
			// Read message with timeout
			msgCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			msg, err := c.reader.FetchMessage(msgCtx)
			cancel()

			if err != nil {
				if err == context.DeadlineExceeded || err == context.Canceled {
					continue
				}
				c.logger.Error("failed to fetch message", zap.Error(err))
				time.Sleep(1 * time.Second)
				continue
			}

			// Process message
			key := string(msg.Key)
			value := msg.Value

			if err := c.handler(ctx, key, value); err != nil {
				c.logger.Error("message handler failed",
					zap.String("key", key),
					zap.Error(err),
				)
				// Continue processing even if handler fails
				// In production, you might want to implement retry logic or dead letter queue
			}

			// Commit offset if not auto-commit
			if !c.autoCommit {
				if err := c.reader.CommitMessages(ctx, msg); err != nil {
					c.logger.Error("failed to commit message",
						zap.String("key", key),
						zap.Error(err),
					)
				}
			}
		}
	}
}

// Stop stops the consumer gracefully.
func (c *Consumer) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return nil
	}

	c.logger.Info("stopping kafka consumer")

	// Cancel context to stop consuming
	c.cancel()

	// Wait for consume goroutine to finish
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()

	// Wait with timeout
	select {
	case <-done:
		c.logger.Info("kafka consumer stopped gracefully")
	case <-time.After(30 * time.Second):
		c.logger.Warn("kafka consumer stop timeout, forcing close")
	}

	// Close reader
	if err := c.reader.Close(); err != nil {
		c.logger.Error("failed to close kafka reader", zap.Error(err))
		return err
	}

	c.running = false
	return nil
}

// Close closes the consumer (implements bootstrap.Resource interface).
func (c *Consumer) Close() error {
	return c.Stop()
}

