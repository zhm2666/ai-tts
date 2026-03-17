package entry

import (
	_interface "ai-transform-backend/interface"
	"ai-transform-backend/message"
	"ai-transform-backend/pkg/config"
	"ai-transform-backend/pkg/constants"
	"ai-transform-backend/pkg/log"
	"ai-transform-backend/pkg/mq/kafka"
	"ai-transform-backend/pkg/storage"
	"context"
	"encoding/json"
	"fmt"
	"github.com/IBM/sarama"
	"net/url"
	"path"
	"strings"
)

type entry struct {
	conf              *config.Config
	log               log.ILogger
	cosStorageFactory storage.StorageFactory
}

func NewEntry(conf *config.Config, log log.ILogger, cosStorageFactory storage.StorageFactory) _interface.ConsumerTask {
	return &entry{
		conf:              conf,
		log:               log,
		cosStorageFactory: cosStorageFactory,
	}
}

func (t *entry) Start(ctx context.Context) {
	cfg := t.conf
	conf := &kafka.ConsumerGroupConfig{
		Config: kafka.Config{
			BrokerList:    cfg.ExternalKafka.Address,
			User:          cfg.ExternalKafka.User,
			Pwd:           cfg.ExternalKafka.Pwd,
			SASLMechanism: cfg.ExternalKafka.SaslMechanism,
			Version:       sarama.V3_7_0_0,
		},
	}
	cg := kafka.NewConsumerGroup(conf, t.log, t.messageHandleFunc)
	cg.Start(ctx, constants.KAFKA_TOPIC_TRANSFORM_WEB_ENTRY, []string{constants.KAFKA_TOPIC_TRANSFORM_WEB_ENTRY})
}
func (t *entry) messageHandleFunc(consumerMessage *sarama.ConsumerMessage) error {
	entryMsg := &message.KafkaMsg{}
	err := json.Unmarshal(consumerMessage.Value, entryMsg)
	if err != nil {
		t.log.Error(err)
		return err
	}
	t.log.DebugF("%+v \n", entryMsg)
	/*
		entryMsg.RecordsID = 2
		entryMsg.OriginalVideoUrl = "https://mediahubdev.0voice.com/ai-transform/inputs/0/0_f52435f782cccf2176d3613ac5122a2c_1766669502.mp4"
		entryMsg.UserID = 0
		entryMsg.SourceLanguage = "zh"
		entryMsg.TargetLanguage = "en"
	*/
	cs := t.cosStorageFactory.CreateStorage()
	dstPath := fmt.Sprintf("%s/%s", constants.INPUTS_DIR, path.Base(entryMsg.OriginalVideoUrl))
	originalVideoUrl, err := url.Parse(entryMsg.OriginalVideoUrl)
	if err != nil {
		t.log.Error(err)
		return err
	}
	objectKey := strings.Trim(originalVideoUrl.Path, "/")
	err = cs.DownloadFile(objectKey, dstPath)
	if err != nil {
		t.log.Error(err)
		return err
	}

	avExtractMsg := entryMsg
	avExtractMsg.SourceFilePath = dstPath
	value, err := json.Marshal(avExtractMsg)
	if err != nil {
		t.log.Error(err)
		return err
	}

	producer := kafka.GetProducer(kafka.Producer)
	msg := &sarama.ProducerMessage{
		Topic: constants.KAFKA_TOPIC_TRANSFORM_AV_EXTRACT,
		Value: sarama.StringEncoder(value),
	}
	_, _, err = producer.SendMessage(msg)
	if err != nil {
		t.log.Error(err)
		return err
	}
	return nil
}
