package controllers

import (
	"ai-transform-backend/pkg/asr"
	"ai-transform-backend/pkg/config"
	"ai-transform-backend/pkg/constants"
	"ai-transform-backend/pkg/log"
	"ai-transform-backend/pkg/storage"
	"fmt"
	"github.com/gin-gonic/gin"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

var (
	REFER_WAV = constants.REFER_WAV
)

type ReferWav struct {
	conf              *config.Config
	log               log.ILogger
	cosStorageFactory storage.StorageFactory
	asrFactory        asr.AsrFactory
}

func NewReferWav(cosStorageFactory storage.StorageFactory, asrFactory asr.AsrFactory, conf *config.Config, log log.ILogger) *ReferWav {
	initReferWavDir(conf)
	return &ReferWav{
		conf:              conf,
		log:               log,
		cosStorageFactory: cosStorageFactory,
		asrFactory:        asrFactory,
	}
}

func initReferWavDir(conf *config.Config) error {
	if conf.Http.Mode == gin.ReleaseMode {
		REFER_WAV = constants.REFER_WAV
	} else if conf.Http.Mode == gin.TestMode {
		REFER_WAV = constants.TEST_REFER_WAV
	} else {
		REFER_WAV = "E:/work/code/2510_vip/GPT-SoVITS-v2pro-20250604/runtime/test-refer"
	}
	_, err := os.Stat(REFER_WAV)
	if os.IsNotExist(err) {
		err = os.MkdirAll(REFER_WAV, os.ModePerm)
		if err != nil {
			return err
		}
	}
	return nil
}

type SaveReferInput struct {
	RecordID        int64                 `form:"record_id" binding:"required"`
	PromptLanguage  string                `form:"prompt_language" binding:"required"`
	ReferFileHeader *multipart.FileHeader `form:"refer_wav_file" binding:"required"`
}

func (c *ReferWav) SaveReferWav(ctx *gin.Context) {
	in := &SaveReferInput{}
	if err := ctx.ShouldBind(in); err != nil {
		c.log.Error(err)
		ctx.JSON(http.StatusBadRequest, gin.H{})
		return
	}
	wavFilePath := fmt.Sprintf("%s/%d.wav", REFER_WAV, in.RecordID)
	promptTextFilePath := fmt.Sprintf("%s/%d.txt", REFER_WAV, in.RecordID)
	err := ctx.SaveUploadedFile(in.ReferFileHeader, wavFilePath)
	if err != nil {
		c.log.Error(err)
		ctx.JSON(http.StatusInternalServerError, gin.H{})
		return
	}

	// 上传音频到COS
	storageAudioPath := fmt.Sprintf("/%s/%s", constants.COS_TMP_REFER, path.Base(wavFilePath))
	s := c.cosStorageFactory.CreateStorage()
	audioUrl, err := s.UploadFromFile(wavFilePath, storageAudioPath)
	if err != nil {
		c.log.Error(err)
		ctx.JSON(http.StatusInternalServerError, gin.H{})
		return
	}
	// 语音识别
	asrText, err := c.getAsrData(audioUrl)
	if err != nil {
		c.log.Error(err)
		ctx.JSON(http.StatusInternalServerError, gin.H{})
		return
	}
	file, err := os.OpenFile(promptTextFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	defer file.Close()
	if err != nil {
		c.log.Error(err)
		ctx.JSON(http.StatusInternalServerError, gin.H{})
		return
	}
	content := fmt.Sprintf("%s\n%s", in.PromptLanguage, asrText)
	_, err = file.WriteString(content)
	if err != nil {
		c.log.Error(err)
		ctx.JSON(http.StatusInternalServerError, gin.H{})
		return
	}
	mp := make(map[string]string, 0)
	mp["refer_wav_path"] = wavFilePath
	mp["prompt_text"] = asrText
	mp["prompt_language"] = in.PromptLanguage
	ctx.JSON(http.StatusOK, mp)
}

type GetReferInput struct {
	RecordID int64 `form:"record_id" binding:"required"`
}

func (c *ReferWav) GetReferInfo(ctx *gin.Context) {
	in := &GetReferInput{}
	err := ctx.ShouldBind(in)
	if err != nil {
		c.log.Error(err)
		ctx.JSON(http.StatusBadRequest, gin.H{})
		return
	}
	wavFilePath := fmt.Sprintf("%s/%d.wav", REFER_WAV, in.RecordID)
	promptTextFilePath := fmt.Sprintf("%s/%d.txt", REFER_WAV, in.RecordID)
	_, err = os.Stat(wavFilePath)
	if os.IsNotExist(err) {
		ctx.JSON(http.StatusOK, gin.H{})
		return
	}
	if err != nil {
		c.log.Error(err)
		ctx.JSON(http.StatusInternalServerError, gin.H{})
		return
	}
	_, err = os.Stat(promptTextFilePath)
	if os.IsNotExist(err) {
		ctx.JSON(http.StatusOK, gin.H{})
		return
	}
	if err != nil {
		c.log.Error(err)
		ctx.JSON(http.StatusInternalServerError, gin.H{})
		return
	}
	contentBytes, err := os.ReadFile(promptTextFilePath)
	if err != nil {
		c.log.Error(err)
		ctx.JSON(http.StatusInternalServerError, gin.H{})
		return
	}
	content := string(contentBytes)
	promptLanguage := strings.Split(content, "\n")[0]
	promptText := strings.Split(content, "\n")[1]
	mp := make(map[string]string, 0)
	mp["refer_wav_path"] = wavFilePath
	mp["prompt_text"] = promptText
	mp["prompt_language"] = promptLanguage
	ctx.JSON(http.StatusOK, mp)
}
func (c *ReferWav) getAsrData(audioUrl string) (string, error) {
	a, err := c.asrFactory.CreateAsr()
	if err != nil {
		c.log.Error(err)
		return "", err
	}
	taskId, err := a.Asr(audioUrl)
	if err != nil {
		c.log.Error(err)
		return "", err
	}
	<-time.After(time.Second * 3)
	result := ""
	status := asr.FAILED
outloop:
	for {
		result, status, err = a.GetAsrResult(taskId)
		if err != nil {
			c.log.Error(err)
			return "", err
		}
		switch status {
		case asr.WAITING:
			<-time.After(time.Second * 3)
			continue
		case asr.DOING:
			<-time.After(time.Second * 1)
			continue
		case asr.SUCCESS:
			break outloop
		case asr.FAILED:
			c.log.Error(err)
			return "", err
		}
	}
	if result != "" {
		list := strings.Split(result, "\n")
		content := ""
		for i := 0; i < len(list); i++ {
			l := strings.Split(list[i], "]")
			if len(l) > 1 {
				content += strings.Trim(l[1], " ")
			} else {
				content += strings.TrimSpace(list[i])
			}
		}
		return content, nil
	}
	return "", err
}
