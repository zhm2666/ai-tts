package main

import (
	"ai-transform-backend/pkg/config"
	"ai-transform-backend/pkg/log"
	"ai-transform-backend/pkg/mq/kafka"
	"context"
	"flag"
	"fmt"
	"github.com/IBM/sarama"
	"os"
	"os/signal"
)

var (
	configFile = flag.String("config", "dev.config.yaml", "config file path")
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Kill, os.Interrupt)
	defer stop()
	flag.Parse()
	config.InitConfig(*configFile)
	cfg := config.GetConfig()
	fmt.Println(cfg)

	log.SetLevel(cfg.Log.Level)
	log.SetOutput(log.GetRotateWriter(cfg.Log.LogPath))
	log.SetPrintCaller(true)
	log.Error("AI Transform Backend started")

	logger := log.NewLogger()
	logger.SetLevel(cfg.Log.Level)
	logger.SetOutput(log.GetRotateWriter(cfg.Log.LogPath))
	logger.SetPrintCaller(true)
	logger.Error("AI Transform Backend started")

	kafkaConf := &kafka.ProducerConfig{
		Config: kafka.Config{
			BrokerList:    cfg.Kafka.Address,
			User:          cfg.Kafka.User,
			Pwd:           cfg.Kafka.Pwd,
			SASLMechanism: cfg.Kafka.SaslMechanism,
			Version:       sarama.V3_7_0_0,
		},
		MaxRetry: cfg.Kafka.MaxRetry,
	}
	kafka.InitKafkaProducer(kafka.Producer, kafkaConf)

	externalKafkaConf := &kafka.ProducerConfig{
		Config: kafka.Config{
			BrokerList:    cfg.ExternalKafka.Address,
			User:          cfg.ExternalKafka.User,
			Pwd:           cfg.ExternalKafka.Pwd,
			SASLMechanism: cfg.ExternalKafka.SaslMechanism,
			Version:       sarama.V3_7_0_0,
		},
		MaxRetry: cfg.ExternalKafka.MaxRetry,
	}
	kafka.InitKafkaProducer(kafka.ExternalProducer, externalKafkaConf)
	// 内部Kafka消费者示例
	go func() {
		conf := &kafka.ConsumerGroupConfig{
			Config: kafka.Config{
				BrokerList:    cfg.Kafka.Address,
				User:          cfg.Kafka.User,
				Pwd:           cfg.Kafka.Pwd,
				SASLMechanism: cfg.Kafka.SaslMechanism,
				Version:       sarama.V3_7_0_0,
			},
		}
		cg := kafka.NewConsumerGroup(conf, logger, func(message *sarama.ConsumerMessage) error {
			fmt.Println(fmt.Sprintf("Received message from internal Kafka: topic=%s, partition=%d, offset=%d, key=%s, value=%s",
				message.Topic, message.Partition, message.Offset, string(message.Key), string(message.Value)))
			return nil
		})
		cg.Start(ctx, "internal-consumer-group", []string{"internal-topic"})
	}()
	go func() {
		conf := &kafka.ConsumerGroupConfig{
			Config: kafka.Config{
				BrokerList:    cfg.ExternalKafka.Address,
				User:          cfg.ExternalKafka.User,
				Pwd:           cfg.ExternalKafka.Pwd,
				SASLMechanism: cfg.ExternalKafka.SaslMechanism,
				Version:       sarama.V3_7_0_0,
			},
		}
		cg := kafka.NewConsumerGroup(conf, logger, func(message *sarama.ConsumerMessage) error {
			fmt.Println(fmt.Sprintf("Received message from external Kafka: topic=%s, partition=%d, offset=%d, key=%s, value=%s",
				message.Topic, message.Partition, message.Offset, string(message.Key), string(message.Value)))
			return nil
		})
		cg.Start(ctx, "external-consumer-group", []string{"external-topic"})
	}()

	for i := 0; i < 10; i++ {
		producer := kafka.GetProducer(kafka.Producer)
		part, offset, err := producer.SendMessage(&sarama.ProducerMessage{
			Topic: "internal-topic",
			Value: sarama.StringEncoder(fmt.Sprintf("hello internal kafka %d", i)),
		})
		fmt.Println(part, offset, err)
	}
	for i := 0; i < 10; i++ {
		producer := kafka.GetProducer(kafka.ExternalProducer)
		part, offset, err := producer.SendMessage(&sarama.ProducerMessage{
			Topic: "external-topic",
			Value: sarama.StringEncoder(fmt.Sprintf("hello external kafka %d", i)),
		})
		fmt.Println(part, offset, err)
	}
	<-ctx.Done()
}
