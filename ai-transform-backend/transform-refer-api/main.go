package main

import (
	"ai-transform-backend/pkg/asr/tasr"
	"ai-transform-backend/pkg/config"
	"ai-transform-backend/pkg/log"
	"ai-transform-backend/pkg/storage/cos"
	"ai-transform-backend/transform-refer-api/controllers"
	"flag"
	"fmt"
	"github.com/gin-gonic/gin"
)

var (
	configFile = flag.String("config", "dev.referapi.config.yaml", "config file path")
)

func main() {
	flag.Parse()
	config.InitConfig(*configFile)
	cfg := config.GetConfig()

	log.SetLevel(cfg.Log.Level)
	log.SetOutput(log.GetRotateWriter(cfg.Log.LogPath))
	log.SetPrintCaller(true)

	logger := log.NewLogger()
	logger.SetLevel(cfg.Log.Level)
	logger.SetOutput(log.GetRotateWriter(cfg.Log.LogPath))
	logger.SetPrintCaller(true)

	csf := cos.NewCosStorageFactory(cfg.Cos.BucketUrl, cfg.Cos.SecretId, cfg.Cos.SecretKey, cfg.Cos.CDNDomain)
	asrFactory := tasr.NewCreateAsrFactory(cfg.Asr.SecretId, cfg.Asr.SecretKey, cfg.Asr.Endpoint, cfg.Asr.Region)
	referWavController := controllers.NewReferWav(csf, asrFactory, cfg, logger)

	gin.SetMode(cfg.Http.Mode)
	r := gin.Default()
	r.GET("/health", func(*gin.Context) {})
	api := r.Group("/api")
	api.POST("/refer/wav", referWavController.SaveReferWav)
	api.GET("/refer/wav", referWavController.GetReferInfo)

	err := r.Run(fmt.Sprintf("%s:%d", cfg.Http.IP, cfg.Http.Port))
	if err != nil {
		log.Fatal(err)
	}
}
