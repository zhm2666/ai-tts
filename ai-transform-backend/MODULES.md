# Transform 模块关联说明文档

## 1. 模块总览

Transform 模块是整个 AI-Transform 系统的核心处理引擎，包含 8 个处理阶段（消费者），通过 Kafka 消息队列进行异步通信。

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Transform 处理流水线                               │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   ┌─────────┐    ┌─────────┐    ┌─────────┐    ┌─────────┐    ┌────────┐ │
│   │  Entry  │───▶│AV Extract│───▶│   ASR   │───▶│ReferWav │───▶│Translate│ │
│   │ (入口)  │    │(音视频分离)│    │ (语音识别) │    │(参考音频) │    │ (翻译)  │ │
│   └────┬────┘    └────┬─────┘    └────┬─────┘    └────┬─────┘    └───┬────┘ │
│        │              │               │               │              │       │
│        │ Kafka       │ Kafka          │ Kafka         │ Kafka        │ Kafka  │
│        │ topic:     │ topic:         │ topic:        │ topic:       │ topic: │
│        │ web_entry  │ av_extract     │ asr           │ refer_wav    │translate│
│        │            │               │               │              │        │
│        │            │               │               │              │        │
│   ┌────▼───────────▼───────────────▼───────────────▼──────────────▼─────┐ │
│   │                                                                       │ │
│   │                     Kafka Internal Topics                             │ │
│   │                                                                       │ │
│   └────┬────────────────────────────────────────────────────────────┬────┘ │
│        │                                                             │      │
│        │ Kafka                                                       │      │
│        │ topic:                                                      │      │
│        │ audio_generation                                            │      │
│        ▼                                                             │      │
│   ┌─────────┐    ┌───────────┐    ┌────────────┐                      │      │
│   │Audio Gen│───▶│AV Synthesis│───▶│Save Result│                      │      │
│   │(音频生成)│    │(音视频合成) │    │(保存结果)  │                      │      │
│   └─────────┘    └───────────┘    └────────────┘                      │      │
│                                                                      │      │
└──────────────────────────────────────────────────────────────────────┘
```

---

## 2. 各模块详细说明

### 2.1 Entry（入口模块）

**文件**: `transform/entry/entry.go`

**职责**: 
- 监听 `transform_web_entry` topic（来自 ExternalKafka）
- 从腾讯云 COS 下载原始视频到本地
- 发送消息到下一阶段

**输入**:
- Kafka 消息（来自 Web API）

**输出**:
- 本地视频文件路径
- 发送消息到 `transform_av_extract` topic

**关键字段**:
```go
// 输入消息
OriginalVideoUrl string  // COS 上的原始视频URL
SourceLanguage   string  // 源语言
TargetLanguage   string  // 目标语言
RecordsID        int64   // 数据库记录ID
UserID           int64   // 用户ID

// 输出字段
SourceFilePath   string  // 下载到本地的视频路径
Filename         string  // 文件名（无扩展名）
```

---

### 2.2 AV Extract（音视频提取模块）

**文件**: `transform/av-extract/av_extract.go`

**职责**:
- 使用 FFmpeg 并行提取视频的音视频流
- 提取音频（AAC 格式）
- 提取视频（保持原始编码）

**输入**:
- `transform_av_extract` topic 消息
- 包含 `SourceFilePath`

**输出**:
- `ExtractVideoPath`: 提取后的视频路径
- `ExtractAudioPath`: 提取后的音频路径
- 发送到 `transform_asr` topic

**技术细节**:
```go
// 并行提取音视频
wg := sync.WaitGroup{}
go func() {
    // 提取音频: ffmpeg -i input.mp4 -vn -acodec copy output.aac
    audioCmd := exec.Command(ffmpeg.FFmpeg, "-i", sourcePath, "-vn", "-acodec", "copy", audioPath)
}()
go func() {
    // 提取视频: ffmpeg -i input.mp4 -an -vcodec copy output.mp4
    videoCmd := exec.Command(ffmpeg.FFmpeg, "-i", sourcePath, "-an", "-vcodec", "copy", videoPath)
}()
wg.Wait()
```

---

### 2.3 ASR（语音识别模块）

**文件**: `transform/asr/asr.go`

**职责**:
- 将音频上传到 COS（供 ASR 服务访问）
- 调用腾讯云 ASR 服务进行语音识别
- 过滤语气词和噪音字符
- 生成 SRT 格式字幕文件

**输入**:
- `transform_asr` topic 消息
- 包含 `ExtractAudioPath`

**输出**:
- `OriginalSrtPath`: 原始字幕文件路径
- 发送到 `transform_refer_wav` topic

**技术细节**:
```go
// 1. 上传音频到COS
audioUrl, err := s.UploadFromFile(asrMsg.ExtractAudioPath, storageAudioPath)

// 2. 调用ASR服务识别
srtContentSlice, err := t.getAsrData(audioUrl)

// 3. 过滤语气词（配置中定义）
modals := ["啊","呃","哎","唉","呢","嗯"]
modalsRegexp := regexp.MustCompile(modals)

// 4. 生成SRT文件
utils.SaveSrt(srtContentSlice, originalSrtPath)
```

**依赖**:
- 腾讯云 ASR 服务
- 配置文件中的 `asr` 配置项

---

### 2.4 Refer Wav（参考音频模块）

**文件**: `transform/refer-wav/refer_wav.go`

**职责**:
- 处理参考音频（用于 AI 语音合成时的音色参考）
- 优先从外部 API 获取已保存的参考音频
- 如果没有，则从原音频中截取一段作为参考

**输入**:
- `transform_refer_wav` topic 消息
- 包含 `OriginalSrtPath` 和 `ExtractAudioPath`

**输出**:
- `ReferWavPath`: 参考音频路径
- `PromptText`: 参考音频对应的文本
- `PromptLanguage`: 参考音频语言
- 发送到 `transform_translate_srt` topic

**处理逻辑**:
```go
// 优先从外部API获取
referWavPath, promptText, promptLanguage, err := t.getReferWav(referMsg.RecordsID)

// 如果没有，则从原音频截取
if referWavPath == "" {
    // 从字幕中找到时长>=6秒的片段
    // ffmpeg -i audio.aac -ss start_time -t 6 -y output.wav
    referWavPath, err = t.getReferInfoFromSrt(originalSrtPath, extractAudioPath, recordsID)
}
```

**依赖**:
- 外部 ReferWav API 服务（`dependOn.referWav.address`）

---

### 2.5 Translate（翻译模块）

**文件**: `transform/translate/translate.go`

**职责**:
- 读取原始 SRT 字幕
- 调用腾讯云 TMT 翻译 API
- 生成翻译后的 SRT 字幕

**输入**:
- `transform_translate_srt` topic 消息
- 包含 `OriginalSrtPath`

**输出**:
- `TranslateSrtPath`: 翻译后的字幕路径
- 发送到 `transform_audio_generation` topic

**技术细节**:
```go
// 批量翻译优化（API限制6000字符）
for i := 0; i < len(srtContentSlice); i += 4 {
    str := srtContentSlice[i+2]
    c := utf8.RuneCountInString(str)
    if count+c >= 6000 {
        // 翻译当前批次
        targetList, err := tmt.TextTranslateBatch(tmpSourceList, sourceLanguage, targetLanguage)
        // 重置
        tmpSourceList = []string{str}
        count = c
    }
}
```

**依赖**:
- 腾讯云 TMT 翻译服务
- 配置文件中的 `tmt` 配置项

---

### 2.6 Audio Generation（音频生成模块）

**文件**: `transform/audio-generation/audio_generation.go`

**职责**:
- 将翻译后的字幕按时间分割（每段5秒）
- 调用外部 AI 语音合成服务（GPT-SoVITS）
- 生成对应音频片段

**输入**:
- `transform_audio_generation` topic 消息
- 包含 `TranslateSrtPath` 和 `ReferWavPath`

**输出**:
- `GenerationAudioDir`: 生成的音频文件目录
- `TranslateSplitSrtPath`: 分割后的字幕路径
- 发送到 `transform_av_synthesis` topic

**技术细节**:
```go
// 1. 分割字幕（每段5秒）
srtContentSlice = splitSrtContent(srtContentSlice, generationMsg.TargetLanguage)

// 2. 并发调用AI服务
pool := go_pool.NewPool(len(executors), executors...)
for i := 0; i < len(srtContentSlice); i += 4 {
    params := audioReasoningParams{
        text:           srtContentSlice[i+2],
        textLanguage:   targetLanguage,
        referWavPath:   referWavPath,
        // ...
    }
    pool.Schedule(newTask(params))
}
pool.WaitAndClose()
```

**依赖**:
- GPT-SoVITS 服务（`dependOn.gpt` 配置的多个地址）
- 协程池控制并发数

---

### 2.7 AV Synthesis（音视频合成模块）

**文件**: `transform/av-synthesis/av_synthesis.go`

**职责**:
- 按时间组合音频片段
- 处理音频延迟对齐
- 合并音视频
- 烧录字幕到视频

**输入**:
- `transform_av_synthesis` topic 消息
- 包含 `GenerationAudioDir` 和 `TranslateSplitSrtPath`

**输出**:
- `OutPutFilePath`: 最终输出视频路径
- 发送到 `transform_save_result` topic

**技术细节**:
```go
// 1. 按2分钟分组
audioGroups := t.groupBySrt(srtContentSlice, sourceDir, "wav")

// 2. 合并音频（分级合并）
audio, err := t.audioMerge(audioGroups, tmpOutputPath, "wav", "mp3")

// 3. 音视频合并
t.avMerge(avSynthesisMsg.ExtractVideoPath, audio.AudioFile, mergeVideo)

// 4. 烧录字幕
t.addSubtitles(mergeVideo, avSynthesisMsg.TranslateSplitSrtPath, videoResultPath)
```

**关键技术**:
- 音频延迟对齐：`ffmpeg -i input.wav -af "adelay=1000"`
- 生成静音填充：`ffmpeg -f lavfi -i anullsrc=r=44100:cl=mono -t 5000ms`
- 字幕烧录：`ffmpeg -i input.mp4 -vf "subtitles=xxx.srt"`

---

### 2.8 Save Result（保存结果模块）

**文件**: `transform/save-result/save_result.go`

**职责**:
- 将最终视频上传到腾讯云 COS
- 更新数据库记录

**输入**:
- `transform_save_result` topic 消息
- 包含 `OutPutFilePath`

**输出**:
- 无（流水线结束）

```go
// 1. 上传到COS
url, err := s.UploadFromFile(saveResultMsg.OutPutFilePath, saveFilePath)

// 2. 更新数据库
recordsData.Update(&data.TransformRecords{
    ID:                 saveResultMsg.RecordsID,
    TranslatedVideoUrl: url,        // 翻译后视频URL
    UpdateAt:           time.Now().Unix(),
    ExpirationAt:       time.Now().Add(time.Hour * 72).Unix(),  // 72小时后过期
})
```

---

## 3. 模块间数据流转

### 3.1 消息字段传递图

```
┌─────────────┐
│ Entry      │
├─────────────┤
│ 输入:       │
│ - Original │
│   VideoUrl │
│ - Source   │
│   Language │
│ - Target   │
│   Language │
│ - RecordsID│
│ - UserID   │
├─────────────┤
│ 输出:       │
│ - Source   │──────────┐
│   FilePath │          │
│ - Filename │          ▼
└─────────────┘    ┌─────────────┐
                   │ AV Extract  │
                   ├─────────────┤
                   │ 输入:       │
                   │ - Source    │
                   │   FilePath  │
                   ├─────────────┤
                   │ 输出:       │
                   │ - Extract   │──────────┐
                   │   VideoPath │          │
                   │ - Extract   │          ▼
                   │   AudioPath │    ┌─────────────┐
                   └─────────────┘    │ ASR         │
                                      ├─────────────┤
                                      │ 输入:       │
                                      │ - Extract   │
                                      │   AudioPath │
                                      ├─────────────┤
                                      │ 输出:       │
                                      │ - Original  │──────────┐
                                      │   SrtPath   │          │
                                      └─────────────┘          ▼
                                                ┌─────────────────┐
                                                │ Refer Wav       │
                                                ├─────────────────┤
                                                │ 输入:           │
                                                │ - OriginalSrt  │
                                                │ - ExtractAudio │
                                                ├─────────────────┤
                                                │ 输出:           │
                                                │ - ReferWavPath │──────────┐
                                                │ - PromptText   │          │
                                                │ - PromptLang   │          ▼
                                                └─────────────────┘    ┌─────────────┐
                                                                        │ Translate   │
                                                                        ├─────────────┤
                                                                        │ 输入:       │
                                                                        │ - Original  │
                                                                        │   SrtPath   │
                                                                        ├─────────────┤
                                                                        │ 输出:       │
                                                                        │ - Translate │──────────┐
                                                                        │   SrtPath   │          │
                                                                        └─────────────┘          ▼
                                                                              ┌─────────────────┐
                                                                              │ Audio Generation │
                                                                              ├──────────────────┤
                                                                              │ 输入:            │
                                                                              │ - TranslateSrt  │
                                                                              │ - ReferWavPath  │
                                                                              ├──────────────────┤
                                                                              │ 输出:            │
                                                                              │ - Generation    │──────────┐
                                                                              │   AudioDir      │          │
                                                                              │ - TranslateSplit│          ▼
                                                                              │   SrtPath       │    ┌─────────────┐
                                                                              └─────────────────┘    │AV Synthesis │
                                                                                                   ├─────────────┤
                                                                                                   │ 输入:        │
                                                                                                   │ - Extract   │
                                                                                                   │   VideoPath │
                                                                                                   │ - Generation│
                                                                                                   │   AudioDir  │
                                                                                                   ├─────────────┤
                                                                                                   │ 输出:        │
                                                                                                   │ - OutPut    │──────────┐
                                                                                                   │   FilePath  │          │
                                                                                                   └─────────────┘          ▼
                                                                                                         ┌─────────────────┐
                                                                                                         │ Save Result     │
                                                                                                         ├─────────────────┤
                                                                                                         │ 输入:           │
                                                                                                         │ - OutPutFilePath│
                                                                                                         ├─────────────────┤
                                                                                                         │ 输出:           │
                                                                                                         │ (无 - 流水线结束)│
                                                                                                         └─────────────────┘
```

---

## 4. Kafka Topics 总结

| Topic | 消费者 | 阶段 | 关键输入 | 关键输出 |
|-------|--------|------|----------|----------|
| `transform_web_entry` | Entry | 1 | OriginalVideoUrl | SourceFilePath |
| `transform_av_extract` | AV Extract | 2 | SourceFilePath | ExtractVideoPath, ExtractAudioPath |
| `transform_asr` | ASR | 3 | ExtractAudioPath | OriginalSrtPath |
| `transform_refer_wav` | Refer Wav | 4 | OriginalSrtPath, ExtractAudioPath | ReferWavPath, PromptText |
| `transform_translate_srt` | Translate | 5 | OriginalSrtPath | TranslateSrtPath |
| `transform_audio_generation` | Audio Gen | 6 | TranslateSrtPath, ReferWavPath | GenerationAudioDir |
| `transform_av_synthesis` | AV Synthesis | 7 | ExtractVideoPath, GenerationAudioDir | OutPutFilePath |
| `transform_save_result` | Save Result | 8 | OutPutFilePath | (结束) |

---

## 5. 模块启动顺序

在 `transform/main.go` 中，所有模块并行启动：

```go
go entry.NewEntry(cfg, logger, csf).Start(ctx)
go av_extract.NewAvExtract(cfg, logger).Start(ctx)
go asr.NewAsr(cfg, logger, csf, data, asrFactory).Start(ctx)
go refer_wav.NewReferWav(cfg, logger).Start(ctx)
go translate.NewTranslate(cfg, logger, csf, data, tf).Start(ctx)
go audio_generation.NewGeneration(cfg, logger).Start(ctx)
go av_synthesis.NewAVSynthesis(cfg, logger).Start(ctx)
go save_result.NewSaveResult(cfg, logger, csf, data).Start(ctx)
```

---

## 6. 依赖关系总结

```
                    ┌──────────────┐
                    │   外部依赖    │
                    ├──────────────┤
                    │ - MySQL      │
                    │ - Kafka      │
                    │ - COS        │
                    │ - 腾讯云ASR  │
                    │ - 腾讯云TMT  │
                    │ - GPT-SoVITS │
                    │ - ReferWavAPI│
                    └──────┬───────┘
                           │
        ┌──────────────────┼──────────────────┐
        │                  │                  │
        ▼                  ▼                  ▼
┌───────────────┐  ┌───────────────┐  ┌───────────────┐
│   Web API     │  │ Transform服务  │  │ ReferWav API │
│ (外部入口)    │  │ (8个消费者)    │  │ (参考音频)   │
└───────────────┘  └───────────────┘  └───────────────┘
```

---

*文档更新时间：2026-03-18*
