# AI-Transform 视频翻译系统说明文档

## 1. 项目概述

**AI-Transform** 是一个智能视频翻译/配音系统，旨在将一种语言的视频自动翻译并配音为另一种语言。主要功能包括：
- 视频音视频分离
- 语音识别 (ASR) 生成字幕
- 机器翻译
- AI 语音合成
- 音视频合成与字幕烧录

## 2. 项目架构

```
ai-transform-master/
├── ai-transform-backend/          # 后端服务
│   ├── main.go                    # 主入口（Kafka消费者示例）
│   ├── transform/                 # 核心转换处理模块
│   │   ├── main.go               # 转换服务主入口（8个消费者任务）
│   │   ├── entry/               # 入口处理 - 从Web接收任务
│   │   ├── av-extract/          # 音视频分离
│   │   ├── asr/                 # 语音识别
│   │   ├── translate/           # 字幕翻译
│   │   ├── refer-wav/           # 参考音频处理
│   │   ├── audio-generation/     # AI音频生成
│   │   ├── av-synthesis/        # 音视频合成
│   │   └── save-result/         # 结果保存
│   ├── transform-web-api/        # Web API服务
│   │   ├── main.go
│   │   ├── controllers/
│   │   ├── routers/
│   │   └── middleware/
│   ├── transform-refer-api/       # 参考音频API服务
│   ├── pkg/                       # 公共包
│   │   ├── asr/                   # ASR封装（腾讯云）
│   │   ├── machine-translate/     # 机器翻译封装（腾讯云TMT）
│   │   ├── storage/               # 对象存储（腾讯云COS）
│   │   ├── mq/kafka/              # Kafka生产者/消费者
│   │   ├── ffmpeg/                # FFmpeg封装
│   │   ├── config/                # 配置管理
│   │   ├── log/                   # 日志系统
│   │   ├── db/mysql/               # MySQL数据库
│   │   ├── utils/                  # 工具函数
│   │   ├── constants/              # 常量定义
│   │   └── errors/                 # 错误处理
│   ├── data/                       # 数据模型
│   ├── message/                    # Kafka消息定义
│   ├── interface/                  # 接口定义
│   ├── dev.config.yaml             # 配置文件
│   └── go.mod
└── ... (其他文件)
```

## 3. 核心技术栈

| 类别 | 技术 |
|------|------|
| **语言** | Go 1.24 |
| **消息队列** | Kafka (IBM sarama) |
| **Web框架** | Gin |
| **ASR服务** | 腾讯云 ASR |
| **机器翻译** | 腾讯云 TMT |
| **对象存储** | 腾讯云 COS |
| **数据库** | MySQL |
| **音视频处理** | FFmpeg |
| **配置管理** | Viper |

## 4. 处理流程

### 4.1 完整流水线（8个Stage）

```
┌─────────────────┐
│  Web Entry      │  接收Web请求，下载视频
│  (入口)         │
└────────┬────────┘
         │ Kafka: transform_web_entry
         ▼
┌─────────────────┐
│  AV Extract     │  音视频分离（视频+音频）
│  (av-extract)   │
└────────┬────────┘
         │ Kafka: transform_av_extract
         ▼
┌─────────────────┐
│  ASR            │  语音识别生成原始字幕
│  (asr)          │
└────────┬────────┘
         │ Kafka: transform_asr
         ▼
┌─────────────────┐
│  Refer Wav      │  处理参考音频
│  (refer-wav)    │
└────────┬────────┘
         │ Kafka: transform_refer_wav
         ▼
┌─────────────────┐
│  Translate      │  翻译字幕
│  (translate)    │
└────────┬────────┘
         │ Kafka: transform_translate_srt
         ▼
┌─────────────────┐
│  Audio Gen      │  AI音频生成
│  (audio-gen)    │
└────────┬────────┘
         │ Kafka: transform_audio_generation
         ▼
┌─────────────────┐
│  AV Synthesis   │  音视频合成+字幕烧录
│  (av-synthesis) │
└────────┬────────┘
         │ Kafka: transform_av_synthesis
         ▼
┌─────────────────┐
│  Save Result    │  上传结果到COS
│  (save-result)  │
└─────────────────┘
```

### 4.2 详细说明

#### Stage 1: 入口 (entry)
- 监听 `transform_web_entry` topic
- 从COS下载原始视频到本地
- 发送消息到下一阶段

#### Stage 2: 音视频提取 (av-extract)
- 使用FFmpeg并行提取：
  - 视频流（保留原视频）
  - 音频流（提取为AAC）
- 监听 `transform_av_extract` topic

#### Stage 3: 语音识别 (asr)
- 将提取的音频上传到COS
- 调用腾讯云ASR服务进行语音识别
- **关键难点**：过滤语气词、噪音字符
- 生成SRT字幕文件
- 监听 `transform_asr` topic

#### Stage 4: 参考音频 (refer-wav)
- 处理参考音频文件
- 用于后续AI语音合成的音色参考
- 监听 `transform_refer_wav` topic

#### Stage 5: 翻译 (translate)
- 读取原始SRT字幕
- **关键难点**：批量翻译优化（每批6000字符）
- 调用腾讯云TMT翻译API
- 生成翻译后的SRT字幕
- 监听 `transform_translate_srt` topic

#### Stage 6: 音频生成 (audio-generation)
- **关键难点**：
  - SRT字幕按时间分割（每段5秒）
  - 并发调用多个GPT-SoVITS服务
  - 使用协程池控制并发数
- 调用外部AI语音合成服务
- 生成对应音频片段
- 监听 `transform_audio_generation` topic

#### Stage 7: 音视频合成 (av-synthesis)
- **核心难点**：
  - 按时间组合音频片段
  - 处理音频延迟对齐
  - 生成静音填充
- 合并音频与视频
- 烧录字幕到视频
- 监听 `transform_av_synthesis` topic

#### Stage 8: 结果保存 (save-result)
- 将最终视频上传到COS
- 更新数据库记录
- 清理本地临时文件

## 5. 关键代码解析

### 5.1 Kafka消息结构

```go
// message/message.go
type KafkaMsg struct {
    RecordsID             int64  // 记录ID
    UserID                int64  // 用户ID
    OriginalVideoUrl     string // 原始视频URL
    SourceLanguage       string // 源语言
    TargetLanguage       string // 目标语言
    SourceFilePath       string // 源文件路径
    Filename             string // 文件名（无扩展名）
    ExtractVideoPath     string // 提取的视频路径
    ExtractAudioPath     string // 提取的音频路径
    OriginalSrtPath      string // 原始字幕路径
    TranslateSrtPath    string // 翻译字幕路径
    TranslateSplitSrtPath string // 分割后字幕路径
    GenerationAudioDir   string // 生成的音频目录
    OutPutFilePath       string // 输出文件路径
    ReferWavPath         string // 参考音频路径
    PromptText           string // 参考音频文本
    PromptLanguage      string // 参考音频语言
}
```

### 5.2 消费者任务接口

```go
// interface/interface.go
type ConsumerTask interface {
    Start(ctx context.Context)
}
```

每个处理模块都实现此接口，监听特定的Kafka topic。

### 5.3 并发控制 - 协程池

```go
// transform/audio-generation/audio_generation.go
pool := go_pool.NewPool(len(executors), executors...)
for i := 0; i < len(srtContentSlice); i += 4 {
    pool.Schedule(newTask(params))
}
pool.WaitAndClose()
```

### 5.4 FFmpeg并行提取

```go
// transform/av-extract/av_extract.go
wg := sync.WaitGroup{}
wg.Add(1)
go func() {
    // 提取音频
    audioCmd := exec.Command(ffmpeg.FFmpeg, "-i", sourcePath, "-vn", "-acodec", "copy", audioPath)
    audioCmd.Run()
}()
wg.Add(1)
go func() {
    // 提取视频
    videoCmd := exec.Command(ffmpeg.FFmpeg, "-i", sourcePath, "-an", "-vcodec", "copy", videoPath)
    videoCmd.Run()
}()
wg.Wait()
```

## 6. 配置说明

### 6.1 配置文件 (dev.config.yaml)

```yaml
http:
  ip: 0.0.0.0
  port: 8081
  mode: debug

mysql:
  dsn: "root:123456@tcp(192.168.239.164:3306)/ai_transform"
  maxLifeTime: 3600
  maxOpenConn: 10
  maxIdleConn: 10

kafka:
  user: admin
  pwd: 123456
  saslMechanism: PLAIN
  address:
    - 192.168.239.164:29092
    - 192.168.239.164:39092
    - 192.168.239.164:49092

cos:
  secretId: "xxx"
  secretKey: "xxx"
  region: "ap-guangzhou"
  bucket: "mediahubdev-1256487221"
  bucketUrl: "https://mediahubdev-1256487221.cos.ap-guangzhou.myqcloud.com"

asr:
  secretId: "xxx"
  secretKey: "xxx"
  endpoint: "asr.tencentcloudapi.com"
  region: "ap-guangzhou"
  modals: ["啊","呃","哎","唉","呢","嗯"]  # 语气词过滤

tmt:
  secretID: "xxx"
  secretKey: "xxx"
  endpoint: "tmt.tencentcloudcloudapi.com"
  region: "ap-guangzhou"

dependOn:
  gpt:  # AI语音合成服务
    - http://localhost:9880
    - http://localhost:9881
  referWav:
    address: "http://localhost:8085"
```

## 7. 重难点分析

### 7.1 音视频同步问题

**问题**：AI生成的音频与原视频时间轴对齐

**解决方案**：
1. 将长字幕片段按5秒为单位分割
2. 生成的音频片段带有固定延迟
3. 使用FFmpeg的`adelay`滤镜进行时间对齐
4. 生成静音填充空隙

```go
// transform/av-synthesis/av_synthesis.go
// 音频延迟处理
cmd := exec.Command(ffmpeg.FFmpeg, "-i", input, "-af", fmt.Sprintf("adelay=%d", adelay), output)
```

### 7.2 ASR结果过滤

**问题**：语音识别结果包含大量语气词和噪音

**解决方案**：使用正则表达式过滤

```go
// transform/asr/asr.go
// 匹配语气词
modals := fmt.Sprintf("[%s]+", strings.Join(t.conf.Asr.Modals, ""))
modalsRegexp := regexp.MustCompile(modals)
// 匹配标点符号组合
p := "[，。！；？：]+[\\p{Han}]{0,1}[，。！；？：]+"
reg := regexp.MustCompile(p)
```

### 7.3 批量翻译优化

**问题**：翻译API有字符限制，需要批量处理

**解决方案**：累积字符数，达到6000字符时批量翻译

```go
// transform/translate/translate.go
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

### 7.4 音频合成策略

**问题**：长音频直接生成质量差

**解决方案**：分级合并策略

1. 第一级：按2分钟分组
2. 第二级：合并所有分组
3. 使用静音填充和延迟对齐

```go
// transform/av-synthesis/av_synthesis.go
minDuration := 2 * 60 * 1000  // 2分钟
for i := 0; i < len(srtContentSlice); i += 4 {
    if end-tmpGroup.ExpectStart < minDuration {
        tmpGroup.Audios = append(tmpGroup.Audios, a)
    } else {
        // 新建分组
    }
}
```

### 7.5 并发控制

**问题**：外部AI服务并发过高会失败

**解决方案**：使用协程池限制并发数

```go
// pkg/go-pool/go_pool.go
pool := go_pool.NewPool(len(executors), executors...)
for i := 0; i < totalTasks; i++ {
    pool.Schedule(newTask(params))
}
pool.WaitAndClose()
```

## 8. 数据库表结构

### transform_records (转换记录表)

| 字段 | 类型 | 说明 |
|------|------|------|
| id | bigint | 主键 |
| user_id | bigint | 用户ID |
| original_video_url | varchar(512) | 原始视频URL |
| source_language | varchar(10) | 源语言 |
| target_language | varchar(10) | 目标语言 |
| original_srt_url | varchar(512) | 原始字幕URL |
| translated_srt_url | varchar(512) | 翻译字幕URL |
| output_video_url | varchar(512) | 输出视频URL |
| status | int | 状态 |
| create_at | bigint | 创建时间 |
| update_at | bigint | 更新时间 |

## 9. 目录结构说明

```
runtime/                    # 运行时目录
├── inputs/                # 输入视频
├── middle/               # 中间产物
│   └── {filename}/
│       ├── audios/       # 生成的音频片段
│       └── tmp/          # 临时文件
├── outputs/              # 最终输出
├── srts/                 # SRT字幕文件
└── refer/                # 参考音频

/ai-transform/            # COS对象存储根目录
├── inputs/              # 输入视频
├── tmp/
│   ├── audios/          # 临时音频
│   └── refer/           # 临时参考音频
├── srts/               # 字幕文件
└── outputs/            # 输出视频
```

## 10. 启动方式

### 10.1 转换服务（主服务）

```bash
cd ai-transform-backend
go run transform/main.go -config dev.config.yaml
```

### 10.2 Web API服务

```bash
cd ai-transform-backend
go run transform-web-api/main.go -config dev.config.yaml
```

### 10.3 参考音频API服务

```bash
cd ai-transform-backend
go run transform-refer-api/main.go -config dev.referapi.config.yaml
```

## 11. 依赖服务

- **Kafka**: 消息队列（内部+外部）
- **MySQL**: 数据持久化
- **腾讯云COS**: 对象存储
- **腾讯云ASR**: 语音识别
- **腾讯云TMT**: 机器翻译
- **GPT-SoVITS**: AI语音合成（本地部署）
- **FFmpeg**: 音视频处理

## 12. 技术亮点

1. **微服务架构**：基于Kafka的异步处理架构
2. **可扩展性**：每个处理阶段独立，可单独扩展
3. **容错处理**：FFmpeg命令重试机制
4. **性能优化**：
   - 协程池控制并发
   - FFmpeg并行提取
   - 批量翻译优化
5. **完善的错误处理**：统一的错误码和日志体系
6. **配置中心**：基于Viper的配置管理

---

*文档生成时间：2026-03-18*
