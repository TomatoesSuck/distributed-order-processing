package amqpretry

import (
	"errors"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
)

func TestPermanent_IsDetectedThroughWrapping(t *testing.T) {
	base := errors.New("bad message")
	assert.True(t, IsPermanent(Permanent(base)))
	assert.Equal(t, base.Error(), Permanent(base).Error())
}

func TestIsPermanent_TransientIsFalse(t *testing.T) {
	assert.False(t, IsPermanent(errors.New("db timeout")))
	assert.False(t, IsPermanent(nil))
}

func TestPermanent_Nil(t *testing.T) {
	assert.Nil(t, Permanent(nil))
}

func TestDecide(t *testing.T) {
	assert.Equal(t, ActionDeadLetter, Decide(Permanent(errors.New("x")), 0, 5))
	assert.Equal(t, ActionRetry, Decide(errors.New("x"), 0, 5))
	assert.Equal(t, ActionRetry, Decide(errors.New("x"), 4, 5))
	assert.Equal(t, ActionDeadLetter, Decide(errors.New("x"), 5, 5))
	assert.Equal(t, ActionDeadLetter, Decide(errors.New("x"), 6, 5))
}

func TestRetryCount_ReadsHeaderTypes(t *testing.T) {
	assert.Equal(t, 0, RetryCount(nil))
	assert.Equal(t, 0, RetryCount(amqp.Table{}))
	assert.Equal(t, 3, RetryCount(amqp.Table{HeaderRetryCount: int32(3)}))
	assert.Equal(t, 7, RetryCount(amqp.Table{HeaderRetryCount: int64(7)}))
	assert.Equal(t, 2, RetryCount(amqp.Table{HeaderRetryCount: 2}))
	assert.Equal(t, 0, RetryCount(amqp.Table{HeaderRetryCount: "garbage"}))
}
