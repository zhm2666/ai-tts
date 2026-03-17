package asr

import (
	"ai-transform-backend/data"
	_interface "ai-transform-backend/interface"
	"ai-transform-backend/message"
	asr2 "ai-transform-backend/pkg/asr"
	"ai-transform-backend/pkg/asr/tasr"
	"ai-transform-backend/pkg/config"
	"ai-transform-backend/pkg/constants"
	"ai-transform-backend/pkg/log"
	"ai-transform-backend/pkg/mq/kafka"
	"ai-transform-backend/pkg/storage"
	"ai-transform-backend/pkg/utils"
	"context"
	"encoding/json"
	"fmt"
	"github.com/IBM/sarama"
	"path"
	"regexp"
	"strings"
	"time"
)

type asr struct {
	conf              *config.Config
	log               log.ILogger
	cosStorageFactory storage.StorageFactory
	data              data.IData
	asrFactory        asr2.AsrFactory
}

func NewAsr(conf *config.Config, log log.ILogger, cosStorageFactory storage.StorageFactory, data data.IData, asrFactory asr2.AsrFactory) _interface.ConsumerTask {
	return &asr{
		conf:              conf,
		log:               log,
		cosStorageFactory: cosStorageFactory,
		data:              data,
		asrFactory:        asrFactory,
	}
}

func (t *asr) Start(ctx context.Context) {
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
	cg.Start(ctx, constants.KAFKA_TOPIC_TRANSFORM_ASR, []string{constants.KAFKA_TOPIC_TRANSFORM_ASR})
}
func (t *asr) messageHandleFunc(consumerMessage *sarama.ConsumerMessage) error {
	asrMsg := &message.KafkaMsg{}
	err := json.Unmarshal(consumerMessage.Value, asrMsg)
	if err != nil {
		t.log.Error(err)
		return err
	}
	t.log.DebugF("%+v \n", asrMsg)
	// 上传录音文件到Cos
	storageAudioPath := fmt.Sprintf("%s/%s", constants.COS_TMP_AUDIO, path.Base(asrMsg.ExtractAudioPath))
	s := t.cosStorageFactory.CreateStorage()
	audioUrl, err := s.UploadFromFile(asrMsg.ExtractAudioPath, storageAudioPath)
	if err != nil {
		t.log.Error(err)
		return err
	}
	// 调用ASR服务进行识别
	srtContentSlice, err := t.getAsrData(audioUrl)
	if err != nil {
		t.log.Error(err)
		return err
	}
	// 将原始的srt文件上传到cos
	originalSrtFilename := fmt.Sprintf("%s_origin.srt", asrMsg.Filename)
	originalSrtPath := fmt.Sprintf("%s/%s", constants.SRTS_DIR, originalSrtFilename)
	err = utils.SaveSrt(srtContentSlice, originalSrtPath)
	if err != nil {
		t.log.Error(err)
		return err
	}
	storageSrtPath := fmt.Sprintf("%s/%s", constants.COS_SRTS, path.Base(originalSrtPath))
	srtUrl, err := s.UploadFromFile(originalSrtPath, storageSrtPath)
	if err != nil {
		t.log.Error(err)
		return err
	}
	recordsData := t.data.NewTransformRecordsData()
	err = recordsData.Update(&data.TransformRecords{
		ID:             asrMsg.RecordsID,
		OriginalSrtUrl: srtUrl,
		UpdateAt:       time.Now().Unix(),
	})
	if err != nil {
		t.log.Error(err)
		return err
	}

	referMsg := asrMsg
	referMsg.OriginalSrtPath = originalSrtPath

	value, err := json.Marshal(referMsg)
	if err != nil {
		t.log.Error(err)
		return err
	}
	producer := kafka.GetProducer(kafka.Producer)
	msg := &sarama.ProducerMessage{
		Topic: constants.KAFKA_TOPIC_TRANSFORM_REFER_WAV,
		Value: sarama.StringEncoder(value),
	}
	_, _, err = producer.SendMessage(msg)
	if err != nil {
		t.log.Error(err)
		return err
	}
	return nil
}

func (t *asr) getAsrData(audioUrl string) ([]string, error) {
	a, err := t.asrFactory.CreateAsr()
	if err != nil {
		t.log.Error(err)
		return nil, err
	}
	taskId, err := a.Asr(audioUrl)
	if err != nil {
		t.log.Error(err)
		return nil, err
	}
	<-time.After(time.Second * 5)
	result := ""
	status := asr2.FAILED
loop:
	for {
		result, status, err = a.GetAsrResult(taskId)
		if err != nil {
			t.log.Error(err)
			return nil, err
		}
		switch status {
		case asr2.WAITING:
			<-time.After(time.Second * 5)
			continue
		case asr2.DOING:
			<-time.After(time.Second)
			continue
		case asr2.SUCCESS:
			break loop
		case asr2.FAILED:
			t.log.Error(err)
			return nil, err
		}
	}
	if result != "" {
		contentSlice := tasr.TenCentAsrToSRT(result)
		// 过滤噪音字符
		contentSlice = t.filterModals(contentSlice)
		return contentSlice, nil
	}
	return nil, err
}
func (t *asr) filterModals(contentSlice []string) []string {
	// 匹配一个汉字加多个中文标点
	modalsStart := "^[\\p{Han}]{1}[，。！；？：]{1,}"
	modalsStartRegexp := regexp.MustCompile(modalsStart)

	// 匹配配置文件当中指定的语气词
	modals := fmt.Sprintf("[%s]+", strings.Join(t.conf.Asr.Modals, ""))
	modalsRegexp := regexp.MustCompile(modals)

	//匹配一个或多个标点+零个或一个汉字+一个或多个标点
	p := "[，。！；？：]+[\\p{Han}]{0,1}[，。！；？：]+"
	reg := regexp.MustCompile(p)

	list := make([]string, 0, len(contentSlice))

	var j = 1
	for i := 0; i < len(contentSlice); i += 4 {
		c := contentSlice[i+2]
		c = modalsStartRegexp.ReplaceAllString(c, "")
		c = modalsRegexp.ReplaceAllString(c, "")
		l := reg.FindAllString(c, -1)
		for _, s := range l {
			firstRune := string([]rune(s)[0])
			c = strings.ReplaceAll(c, s, firstRune)
		}
		if len(c) == 0 {
			continue
		}
		list = append(list, fmt.Sprintf("%d", j), contentSlice[i+1], c, "")
		j++
	}
	return list
}
