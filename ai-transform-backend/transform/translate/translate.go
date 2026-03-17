package translate

import (
	"ai-transform-backend/data"
	_interface "ai-transform-backend/interface"
	"ai-transform-backend/message"
	"ai-transform-backend/pkg/config"
	"ai-transform-backend/pkg/constants"
	"ai-transform-backend/pkg/log"
	machine_translate "ai-transform-backend/pkg/machine-translate"
	"ai-transform-backend/pkg/mq/kafka"
	"ai-transform-backend/pkg/storage"
	"ai-transform-backend/pkg/utils"
	"context"
	"encoding/json"
	"fmt"
	"github.com/IBM/sarama"
	"io"
	"os"
	"path"
	"strings"
	"time"
	"unicode/utf8"
)

type translate struct {
	conf              *config.Config
	log               log.ILogger
	cosStorageFactory storage.StorageFactory
	data              data.IData
	tf                machine_translate.TranslatorFactory
}

func NewTranslate(conf *config.Config, log log.ILogger, cosStorageFactory storage.StorageFactory, data data.IData, tf machine_translate.TranslatorFactory) _interface.ConsumerTask {
	return &translate{
		conf:              conf,
		log:               log,
		tf:                tf,
		cosStorageFactory: cosStorageFactory,
		data:              data,
	}
}

func (t *translate) Start(ctx context.Context) {
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
	cg.Start(ctx, constants.KAFKA_TOPIC_TRANSFORM_TRANSLATE_SRT, []string{constants.KAFKA_TOPIC_TRANSFORM_TRANSLATE_SRT})
}
func (t *translate) messageHandleFunc(consumerMessage *sarama.ConsumerMessage) error {
	translateMsg := &message.KafkaMsg{}
	err := json.Unmarshal(consumerMessage.Value, translateMsg)
	if err != nil {
		t.log.Error(err)
		return err
	}
	t.log.DebugF("%+v \n", translateMsg)
	originalSrtPath := translateMsg.OriginalSrtPath
	file, err := os.Open(originalSrtPath)
	if err != nil {
		t.log.Error(err)
		return err
	}
	defer file.Close()
	srtContentBytes, err := io.ReadAll(file)
	if err != nil {
		t.log.Error(err)
		return err
	}
	srtContent := string(srtContentBytes)
	srtContentSlice := strings.Split(srtContent, "\n")
	err = t.translateSrtContent(srtContentSlice, translateMsg.SourceLanguage, translateMsg.TargetLanguage)
	if err != nil {
		t.log.Error(err)
		return err
	}
	translateSrtFilename := fmt.Sprintf("%s_translate.srt", translateMsg.Filename)
	translateSrtPath := fmt.Sprintf("%s/%s", constants.SRTS_DIR, translateSrtFilename)
	err = utils.SaveSrt(srtContentSlice, translateSrtPath)
	if err != nil {
		t.log.Error(err)
		return err
	}
	s := t.cosStorageFactory.CreateStorage()
	storageSrtPath := fmt.Sprintf("%s/%s", constants.COS_SRTS, path.Base(translateSrtPath))
	srtUrl, err := s.UploadFromFile(translateSrtPath, storageSrtPath)
	if err != nil {
		t.log.Error(err)
		return err
	}
	recordsData := t.data.NewTransformRecordsData()
	err = recordsData.Update(&data.TransformRecords{
		ID:               translateMsg.RecordsID,
		TranslatedSrtUrl: srtUrl,
		UpdateAt:         time.Now().Unix(),
	})
	if err != nil {
		t.log.Error(err)
		return err
	}

	generationMsg := translateMsg
	generationMsg.TranslateSrtPath = translateSrtPath

	value, err := json.Marshal(translateMsg)
	if err != nil {
		t.log.Error(err)
		return err
	}
	producer := kafka.GetProducer(kafka.Producer)
	msg := &sarama.ProducerMessage{
		Topic: constants.KAFKA_TOPIC_TRANSFORM_AUDIO_GENERATION,
		Value: sarama.StringEncoder(value),
	}
	_, _, err = producer.SendMessage(msg)
	if err != nil {
		t.log.Error(err)
		return err
	}
	return nil
}

func (t *translate) translateSrtContent(srtContentSlice []string, sourceLanguage, targetLanguage string) error {
	tmt, err := t.tf.CreateTranslator()
	if err != nil {
		t.log.Error(err)
		return err
	}
	resultList := make([]*string, 0)
	count := 0
	tmpSourceList := make([]string, 0)
	for i := 0; i < len(srtContentSlice); i += 4 {
		str := srtContentSlice[i+2]
		c := utf8.RuneCountInString(str)
		if count+c >= 6000 {
			targetList, err := tmt.TextTranslateBatch(tmpSourceList, sourceLanguage, targetLanguage)
			if err != nil {
				t.log.Error(err)
				return err
			}
			resultList = append(resultList, targetList...)
			count = c
			tmpSourceList = []string{str}
		} else {
			tmpSourceList = append(tmpSourceList, str)
			count += c
		}
		if i == len(srtContentSlice)-4 {
			targetList, err := tmt.TextTranslateBatch(tmpSourceList, sourceLanguage, targetLanguage)
			if err != nil {
				t.log.Error(err)
				return err
			}
			resultList = append(resultList, targetList...)
		}
	}
	for i := 0; i < len(resultList); i++ {
		srtContentSlice[4*i+2] = *resultList[i]
	}
	return nil
}
