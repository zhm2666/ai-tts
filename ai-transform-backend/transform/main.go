package main

import (
	data2 "ai-transform-backend/data"
	"ai-transform-backend/pkg/asr/tasr"
	"ai-transform-backend/pkg/config"
	"ai-transform-backend/pkg/constants"
	"ai-transform-backend/pkg/db/mysql"
	"ai-transform-backend/pkg/log"
	"ai-transform-backend/pkg/machine-translate/tmt"
	"ai-transform-backend/pkg/mq/kafka"
	"ai-transform-backend/pkg/storage/cos"
	"ai-transform-backend/pkg/utils"
	"ai-transform-backend/transform/asr"
	audio_generation "ai-transform-backend/transform/audio-generation"
	av_extract "ai-transform-backend/transform/av-extract"
	av_synthesis "ai-transform-backend/transform/av-synthesis"
	"ai-transform-backend/transform/entry"
	refer_wav "ai-transform-backend/transform/refer-wav"
	save_result "ai-transform-backend/transform/save-result"
	"ai-transform-backend/transform/translate"
	"context"
	"flag"
	"github.com/IBM/sarama"
	"os"
	"os/signal"
)

var (
	configFile = flag.String("config", "dev.config.yaml", "config file path")
)

func main() {
	flag.Parse()

	err := utils.CreateDirIfNotExists(
		constants.INPUTS_DIR,
		constants.OUTPUTSDIR,
		constants.MIDDLE_DIR,
		constants.SRTS_DIR,
		constants.REFER_WAV)
	if err != nil {
		log.Fatal(err)
	}

	config.InitConfig(*configFile)
	cfg := config.GetConfig()

	log.SetLevel(cfg.Log.Level)
	log.SetOutput(log.GetRotateWriter(cfg.Log.LogPath))
	log.SetPrintCaller(true)

	logger := log.NewLogger()
	logger.SetLevel(cfg.Log.Level)
	logger.SetOutput(log.GetRotateWriter(cfg.Log.LogPath))
	logger.SetPrintCaller(true)
	mysql.InitMysql(cfg)
	data := data2.NewData(mysql.GetDB())

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
	csf := cos.NewCosStorageFactory(cfg.Cos.BucketUrl, cfg.Cos.SecretId, cfg.Cos.SecretKey, cfg.Cos.CDNDomain)
	asrFactory := tasr.NewCreateAsrFactory(cfg.Asr.SecretId, cfg.Asr.SecretKey, cfg.Asr.Endpoint, cfg.Asr.Region)
	tf := tmt.NewCreateTmtFactory(cfg.Tmt.SecretID, cfg.Tmt.SecretKey, cfg.Tmt.Endpoint, cfg.Tmt.Region)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer stop()
	go entry.NewEntry(cfg, logger, csf).Start(ctx)
	go av_extract.NewAvExtract(cfg, logger).Start(ctx)
	go asr.NewAsr(cfg, logger, csf, data, asrFactory).Start(ctx)
	go refer_wav.NewReferWav(cfg, logger).Start(ctx)
	go translate.NewTranslate(cfg, logger, csf, data, tf).Start(ctx)
	go audio_generation.NewGeneration(cfg, logger).Start(ctx)
	go av_synthesis.NewAVSynthesis(cfg, logger).Start(ctx)
	go save_result.NewSaveResult(cfg, logger, csf, data).Start(ctx)
	<-ctx.Done()
}
