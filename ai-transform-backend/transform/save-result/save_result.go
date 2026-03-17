package save_result

import (
	"ai-transform-backend/data"
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
	"path"
	"time"
)

type saveResult struct {
	conf              *config.Config
	log               log.ILogger
	cosStorageFactory storage.StorageFactory
	data              data.IData
}

func NewSaveResult(conf *config.Config, log log.ILogger, cosStorageFactory storage.StorageFactory, data data.IData) _interface.ConsumerTask {
	return &saveResult{
		conf:              conf,
		log:               log,
		cosStorageFactory: cosStorageFactory,
		data:              data,
	}
}

func (t *saveResult) Start(ctx context.Context) {
	cfg := t.conf
	conf := &kafka.ConsumerGroupConfig{
		Config: kafka.Config{
			BrokerList:    cfg.Kafka.Address,
			User:          cfg.Kafka.User,
			Pwd:           cfg.Kafka.Pwd,
			SASLMechanism: cfg.Kafka.SaslMechanism,
			Version:       sarama.V3_7_0_0,
		},
	}
	cg := kafka.NewConsumerGroup(conf, t.log, t.messageHandleFunc)
	cg.Start(ctx, constants.KAFKA_TOPIC_TRANSFORM_SAVE_RESULT, []string{constants.KAFKA_TOPIC_TRANSFORM_SAVE_RESULT})
}
func (t *saveResult) messageHandleFunc(consumerMessage *sarama.ConsumerMessage) error {
	saveResultMsg := &message.KafkaMsg{}
	err := json.Unmarshal(consumerMessage.Value, saveResultMsg)
	if err != nil {
		t.log.Error(err)
		return err
	}
	s := t.cosStorageFactory.CreateStorage()
	saveFilePath := fmt.Sprintf("/%s/%s", constants.COS_OUTPUT, path.Base(saveResultMsg.OutPutFilePath))
	url, err := s.UploadFromFile(saveResultMsg.OutPutFilePath, saveFilePath)
	if err != nil {
		t.log.Error(err)
		return err
	}
	recordsData := t.data.NewTransformRecordsData()
	err = recordsData.Update(&data.TransformRecords{
		ID:                 saveResultMsg.RecordsID,
		TranslatedVideoUrl: url,
		UpdateAt:           time.Now().Unix(),
		ExpirationAt:       time.Now().Add(time.Hour * 72).Unix(),
	})
	if err != nil {
		t.log.Error(err)
		return err
	}
	return err
}
