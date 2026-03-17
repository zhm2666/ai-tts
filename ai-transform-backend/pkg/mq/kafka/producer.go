package kafka

import "github.com/IBM/sarama"

const (
	Producer         = "kafka_producer"
	ExternalProducer = "external_kafka_producer"
)

var __producerMap = map[string]sarama.SyncProducer{}

type ProducerConfig struct {
	Config
	MaxRetry int
}
type Config struct {
	BrokerList    []string
	User          string
	Pwd           string
	SASLMechanism string
	Version       sarama.KafkaVersion
}

func InitKafkaProducer(key string, producerConfig *ProducerConfig) error {
	var err error
	__producerMap[key], err = newSyncProducer(producerConfig.BrokerList, producerConfig.User, producerConfig.Pwd, producerConfig.SASLMechanism, producerConfig.Version, producerConfig.MaxRetry)
	return err
}
func newSyncProducer(brokerList []string, user, pwd, saslMechanism string, version sarama.KafkaVersion, maxRetry int) (sarama.SyncProducer, error) {
	config := sarama.NewConfig()
	config.Version = version
	// 等待所有副本都保存成功后的响应
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = maxRetry
	config.Producer.Return.Successes = true
	config.Net.SASL.Enable = true
	config.Net.SASL.User = user
	config.Net.SASL.Password = pwd
	config.Net.SASL.Mechanism = sarama.SASLMechanism(saslMechanism)
	producer, err := sarama.NewSyncProducer(brokerList, config)
	if err != nil {
		return nil, err
	}
	return producer, nil
}
func GetProducer(key string) sarama.SyncProducer {
	return __producerMap[key]
}
