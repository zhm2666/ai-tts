package main

import (
	data2 "ai-transform-backend/data"
	"ai-transform-backend/pkg/config"
	"ai-transform-backend/pkg/db/mysql"
	"ai-transform-backend/pkg/log"
	"ai-transform-backend/pkg/mq/kafka"
	"ai-transform-backend/transform-web-api/controllers"
	"ai-transform-backend/transform-web-api/middleware"
	"ai-transform-backend/transform-web-api/routers"
	"flag"
	"fmt"
	"github.com/IBM/sarama"
	"github.com/gin-gonic/gin"
	"net/http"
)

var (
	configFile = flag.String("config", "dev.config.yaml", "config file path")
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

	cosUploadController := controllers.NewCosUpload(cfg, logger)
	transformController := controllers.NewTransform(cfg, logger, data)

	gin.SetMode(cfg.Http.Mode)
	r := gin.Default()
	r.Use(middleware.Cors())
	r.GET("/health", func(*gin.Context) {})
	api := r.Group("/api")
	api.Use(middleware.Auth())
	routers.InitCosUploadRouters(api, cosUploadController)
	routers.InitTransformRouters(api, transformController)

	fs := http.FileServer(http.Dir("www"))
	r.NoRoute(func(c *gin.Context) {
		fs.ServeHTTP(c.Writer, c.Request)
	})
	r.GET("/", func(c *gin.Context) {
		http.ServeFile(c.Writer, c.Request, "www/index.html")
	})

	err := r.Run(fmt.Sprintf("%s:%d", cfg.Http.IP, cfg.Http.Port))
	if err != nil {
		log.Fatal(err)
	}
}
