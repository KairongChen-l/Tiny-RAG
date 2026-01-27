package circuitbreaker

import (
	"errors"
	"sync"
	"time"
)

// State represents the circuit breaker state.
type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig holds circuit breaker configuration.
type CircuitBreakerConfig struct {
	FailureThreshold int           // Number of failures before opening (default: 5)
	SuccessThreshold int           // Number of successes to close from half-open (default: 2)
	Timeout          time.Duration // Time to wait before attempting half-open (default: 60s)
}

// DefaultCircuitBreakerConfig returns default circuit breaker configuration.
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:           60 * time.Second,
	}
}

// CircuitBreaker implements a simple circuit breaker pattern.
type CircuitBreaker struct {
	mu sync.RWMutex

	config CircuitBreakerConfig

	state         State
	failureCount  int
	successCount  int
	lastFailTime  time.Time
	lastStateTime time.Time
}

// NewCircuitBreaker creates a new circuit breaker.
func NewCircuitBreaker(cfg CircuitBreakerConfig) *CircuitBreaker {
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = 5
	}
	if cfg.SuccessThreshold <= 0 {
		cfg.SuccessThreshold = 2
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 60 * time.Second
	}

	return &CircuitBreaker{
		config:        cfg,
		state:        StateClosed,
		lastStateTime: time.Now(),
	}
}

// State returns the current circuit breaker state.
func (cb *CircuitBreaker) State() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Execute executes a function with circuit breaker protection.
var ErrCircuitOpen = errors.New("circuit breaker is open")

func (cb *CircuitBreaker) Execute(fn func() error) error {
	cb.mu.Lock()

	// Check if we should transition states (e.g., open -> half-open after timeout)
	cb.updateState()

	state := cb.state
	cb.mu.Unlock()

	switch state {
	case StateOpen:
		return ErrCircuitOpen
	case StateHalfOpen:
		// Allow one attempt
		err := fn()
		cb.mu.Lock()
		if err != nil {
			// Failure in half-open opens circuit again
			cb.state = StateOpen
			cb.failureCount = cb.config.FailureThreshold
			cb.lastFailTime = time.Now()
			cb.lastStateTime = time.Now()
			cb.successCount = 0
		} else {
			// Success in half-open increments success count
			cb.successCount++
			if cb.successCount >= cb.config.SuccessThreshold {
				// Close circuit
				cb.state = StateClosed
				cb.failureCount = 0
				cb.successCount = 0
				cb.lastStateTime = time.Now()
			}
		}
		cb.mu.Unlock()
		return err
	case StateClosed:
		// Normal operation
		err := fn()
		cb.mu.Lock()
		if err != nil {
			cb.failureCount++
			cb.lastFailTime = time.Now()
			if cb.failureCount >= cb.config.FailureThreshold {
				// Open circuit
				cb.state = StateOpen
				cb.lastStateTime = time.Now()
			}
		} else {
			// Reset failure count on success
			cb.failureCount = 0
		}
		cb.mu.Unlock()
		return err
	default:
		return errors.New("unknown circuit breaker state")
	}
}

// updateState updates the circuit breaker state based on time and thresholds.
// Must be called with lock held.
func (cb *CircuitBreaker) updateState() {
	now := time.Now()

	switch cb.state {
	case StateOpen:
		// Check if timeout has passed to enter half-open
		if now.Sub(cb.lastStateTime) >= cb.config.Timeout {
			cb.state = StateHalfOpen
			cb.lastStateTime = now
			cb.successCount = 0
		}
	case StateHalfOpen:
		// State transitions handled in Execute
	case StateClosed:
		// State transitions handled in Execute
	}
}

