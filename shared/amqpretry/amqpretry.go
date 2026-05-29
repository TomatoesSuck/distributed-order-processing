// Package amqpretry holds the broker-agnostic decision logic for what to do
// with a failed AMQP delivery: retry it (via a TTL queue) or dead-letter it.
// Kept pure (no *amqp.Channel, no I/O) so it is unit-tested without a broker.
package amqpretry

import (
	"errors"

	amqp "github.com/rabbitmq/amqp091-go"
)

// HeaderRetryCount tracks how many times a message has gone through the retry
// queue. Read on consume, incremented on each retry republish.
const HeaderRetryCount = "x-retry-count"

// RetryExchange is the direct exchange that retry queues bind to. A message
// republished here under its ORIGINAL routing key lands in the matching
// <queue>.retry queue, waits out RetryTTLMillis, then dead-letters back to the
// original exchange under the same routing key.
const RetryExchange = "saga.retry"

// RetryTTLMillis is the fixed backoff a message waits in the retry queue before
// being redelivered to the main queue. Single source of truth for topology +
// tests.
const RetryTTLMillis = 5000

type permanentError struct{ err error }

func (e permanentError) Error() string { return e.err.Error() }
func (e permanentError) Unwrap() error { return e.err }

// Permanent marks an error the consumer must never retry (malformed body,
// unknown routing key) — retrying can only fail again. Returns nil for nil.
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return permanentError{err}
}

// IsPermanent reports whether err (or anything it wraps) was marked Permanent.
func IsPermanent(err error) bool {
	var pe permanentError
	return errors.As(err, &pe)
}

// Action is the routing decision for a failed delivery.
type Action int

const (
	ActionRetry Action = iota
	ActionDeadLetter
)

// Decide picks retry vs dead-letter. Permanent errors always dead-letter;
// transient errors retry until retryCount reaches maxRetries, then dead-letter.
func Decide(err error, retryCount, maxRetries int) Action {
	if IsPermanent(err) || retryCount >= maxRetries {
		return ActionDeadLetter
	}
	return ActionRetry
}

// RetryCount reads HeaderRetryCount from AMQP headers (0 if absent/malformed).
// amqp091 may decode integer headers as int32, int64, or int.
func RetryCount(h amqp.Table) int {
	switch v := h[HeaderRetryCount].(type) {
	case int32:
		return int(v)
	case int64:
		return int(v)
	case int:
		return v
	default:
		return 0
	}
}
