package controllers

import (
	"ai-transform-backend/data"
	"ai-transform-backend/message"
	"ai-transform-backend/pkg/config"
	"ai-transform-backend/pkg/constants"
	"ai-transform-backend/pkg/log"
	"ai-transform-backend/pkg/mq/kafka"
	"encoding/json"
	"github.com/IBM/sarama"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

type Transform struct {
	conf *config.Config
	log  log.ILogger
	data data.IData
}

func NewTransform(conf *config.Config, log log.ILogger, data data.IData) *Transform {
	return &Transform{
		conf: conf,
		log:  log,
		data: data,
	}
}

type transInfo struct {
	ProjectName       string `form:"project_name" binding:"required"`
	OriginalLanguage  string `form:"original_language" binding:"required"`
	TranslateLanguage string `form:"translate_language" binding:"required"`
	FileUrl           string `form:"file_url",binding:"required,url"`
}

func (c *Transform) Translate(ctx *gin.Context) {
	userID, _ := ctx.Get("User.ID")
	ti := &transInfo{}
	err := ctx.ShouldBind(ti)
	if err != nil {
		c.log.Error(err)
		ctx.JSON(http.StatusBadRequest, gin.H{})
		return
	}
	entity := &data.TransformRecords{
		UserID:             userID.(int64),
		ProjectName:        ti.ProjectName,
		OriginalLanguage:   ti.OriginalLanguage,
		TranslatedLanguage: ti.TranslateLanguage,
		OriginalVideoUrl:   ti.FileUrl,
		CreateAt:           time.Now().Unix(),
		UpdateAt:           time.Now().Unix(),
	}
	recordData := c.data.NewTransformRecordsData()
	err = recordData.Add(entity)
	if err != nil {
		c.log.Error(err)
		ctx.JSON(http.StatusInternalServerError, gin.H{})
		return
	}
	// 将消息推送到kafka
	entryMsg := message.KafkaMsg{
		RecordsID:        entity.ID,
		UserID:           userID.(int64),
		OriginalVideoUrl: ti.FileUrl,
		SourceLanguage:   ti.OriginalLanguage,
		TargetLanguage:   ti.TranslateLanguage,
	}
	value, err := json.Marshal(entryMsg)
	if err != nil {
		c.log.Error(err)
		ctx.JSON(http.StatusInternalServerError, gin.H{})
		return
	}
	producer := kafka.GetProducer(kafka.ExternalProducer)
	msg := &sarama.ProducerMessage{
		Topic: constants.KAFKA_TOPIC_TRANSFORM_WEB_ENTRY,
		Value: sarama.StringEncoder(value),
	}
	_, _, err = producer.SendMessage(msg)
	if err != nil {
		c.log.Error(err)
		ctx.JSON(http.StatusInternalServerError, gin.H{})
		return
	}
}

type record struct {
	ID                 int64  `json:"id"`
	ProjectName        string `json:"project_name"`
	OriginalLanguage   string `json:"original_language"`
	TranslatedLanguage string `json:"translated_language"`
	OriginalVideoUrl   string `json:"original_video_url"`
	TranslatedVideoUrl string `json:"translated_video_url"`
	ExpirationAt       int64  `json:"expiration_at"`
	CreateAt           int64  `json:"create_at"`
}

func (c *Transform) GetRecords(ctx *gin.Context) {
	userID, _ := ctx.Get("User.ID")
	recordsData := c.data.NewTransformRecordsData()
	list, err := recordsData.GetByUserID(userID.(int64))
	if err != nil {
		c.log.Error(err)
		ctx.JSON(http.StatusInternalServerError, gin.H{})
		return
	}
	records := make([]record, len(list))
	for index, l := range list {
		records[index].ID = l.ID
		records[index].ProjectName = l.ProjectName
		records[index].OriginalLanguage = l.OriginalLanguage
		records[index].TranslatedLanguage = l.TranslatedLanguage
		records[index].OriginalVideoUrl = l.OriginalVideoUrl
		records[index].TranslatedVideoUrl = l.TranslatedVideoUrl
		records[index].ExpirationAt = l.ExpirationAt
		records[index].CreateAt = l.CreateAt
	}
	ctx.JSON(http.StatusOK, records)
	return
}
