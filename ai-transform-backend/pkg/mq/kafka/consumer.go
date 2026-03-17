package kafka

import (
	"ai-transform-backend/pkg/log"
	"context"
	"github.com/IBM/sarama"
)

type ConsumerGroupConfig struct {
	Config
}

type ConsumerGroup interface {
	Start(ctx context.Context, groupID string, topics []string)
}
type MessageHandleFunc func(message *sarama.ConsumerMessage) error
type consumerGroup struct {
	log               log.ILogger
	conf              *ConsumerGroupConfig
	messageHandleFunc MessageHandleFunc
}

func NewConsumerGroup(config *ConsumerGroupConfig, log log.ILogger, messageHandleFunc MessageHandleFunc) ConsumerGroup {
	return &consumerGroup{
		log:               log,
		conf:              config,
		messageHandleFunc: messageHandleFunc,
	}
}
func (cg *consumerGroup) Start(ctx context.Context, groupID string, topics []string) {
	config := sarama.NewConfig()
	config.Version = cg.conf.Version
	config.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}
	config.Consumer.Offsets.Initial = sarama.OffsetOldest
	config.Net.SASL.Enable = true
	config.Net.SASL.User = cg.conf.User
	config.Net.SASL.Password = cg.conf.Pwd
	config.Net.SASL.Mechanism = sarama.SASLMechanism(cg.conf.SASLMechanism)
	client, err := sarama.NewConsumerGroup(cg.conf.BrokerList, groupID, config)
	if err != nil {
		cg.log.Error(err)
		return
	}
	cgh := &consumerGroupHandler{
		messageHandleFunc: cg.messageHandleFunc,
		log:               cg.log,
	}
	for {
		if err := client.Consume(ctx, topics, cgh); err != nil {
			if err == sarama.ErrClosedConsumerGroup {
				return
			}
			cg.log.Error(err)
		}
		if ctx.Err() != nil {
			return
		}
	}
}

type consumerGroupHandler struct {
	messageHandleFunc MessageHandleFunc
	log               log.ILogger
}

func (cgh *consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	return nil
}
func (cgh *consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}
func (cgh *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case message, ok := <-claim.Messages():
			{
				if !ok {
					log.Info("message channel closed")
					return nil
				}
				err := cgh.messageHandleFunc(message)
				if err != nil {
					cgh.log.Error(err)
				}
				session.MarkMessage(message, "")
			}
		case <-session.Context().Done():
			return nil
		}
	}
}
