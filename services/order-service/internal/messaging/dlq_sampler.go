package messaging

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/TomatoesSuck/distributed-order-processing/shared/observability"
)

const dlqSampleInterval = 15 * time.Second

// StartDLQSampler periodically reads the message count of dlqName via a passive
// queue declare and publishes it to the dlq_messages gauge. Opens a fresh
// channel per sample (every 15s, negligible cost) so a transient channel error
// never wedges the sampler.
func StartDLQSampler(ctx context.Context, mq *MQ, dlqName string, logger *zap.Logger) {
	go func() {
		t := time.NewTicker(dlqSampleInterval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				ch, err := mq.Channel()
				if err != nil {
					logger.Warn("dlq sampler: open channel", zap.Error(err))
					continue
				}
				q, err := ch.QueueDeclarePassive(dlqName, true, false, false, false, nil)
				ch.Close()
				if err != nil {
					logger.Warn("dlq sampler: passive declare", zap.String("queue", dlqName), zap.Error(err))
					continue
				}
				observability.DLQMessages.WithLabelValues(dlqName).Set(float64(q.Messages))
			}
		}
	}()
}
