# AI-Transform 视频翻译系统 - 新手入门指南

## 前言

本文档面向**第一次接触该项目**的开发者，提供从零开始的学习路径和实践指南。

---

## 第一阶段：环境准备 (1-2小时)

### 1.1 依赖服务清单

在运行项目前，需要准备以下服务：

| 服务 | 作用 | 端口 | 备注 |
|------|------|------|------|
| **MySQL** | 数据存储 | 3306 | 需要创建 `ai_transform` 数据库 |
| **Kafka** | 消息队列 | 29092/39092/49092 | 3节点集群 |
| **腾讯云COS** | 对象存储 | - | 需要开通ASR、TMT、COS服务 |
| **GPT-SoVITS** | AI语音合成 | 9880/9881 | 本地部署 |
| **FFmpeg** | 音视频处理 | - | 需要添加到PATH |

### 1.2 本地环境搭建

#### 1.2.1 安装 Go 1.24+

```bash
# 检查Go版本
go version

# 如果未安装，下载安装
# https://go.dev/dl/
```

#### 1.2.2 安装 FFmpeg

```bash
# Windows (使用 Chocolatey)
choco install ffmpeg

# Linux
sudo apt install ffmpeg

# macOS
brew install ffmpeg
```

#### 1.2.3 安装 MySQL

```bash
# Windows (使用 MySQL Installer)
# Linux
sudo apt install mysql-server

# 创建数据库
CREATE DATABASE ai_transform;
```

#### 1.2.4 安装 Kafka (开发环境)

```bash
# 使用 Docker Compose 快速启动
# 创建 docker-compose.yaml
version: '3'
services:
  zookeeper:
    image: confluentinc/cp-zookeeper:7.5.0
    environment:
      ZOOKEEPER_CLIENT_PORT: 2181
  kafka:
    image: confluentinc/cp-kafka:7.5.0
    ports:
      - "29092:29092"
      - "39092:39092"
      - "49092:49092"
    environment:
      KAFKA_BROKER_ID: 0
      KAFKA_ZOOKEEPER_CONNECT: zookeeper:2181
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://localhost:29092,PLAINTEXT_HOST://localhost:9092
      KAFKA_LISTENER_SECURITY_PROTOCOL_MAP: PLAINTEXT:PLAINTEXT,PLAINTEXT_HOST:PLAINTEXT
      KAFKA_INTER_BROKER_LISTENER_NAME: PLAINTEXT
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 1
```

```bash
# 启动
docker-compose up -d
```

### 1.3 克隆项目

```bash
git clone <project-url>
cd ai-transform-master/ai-transform-backend
go mod download
```

### 1.4 配置修改

复制并修改配置文件：

```bash
cp dev.config.yaml prod.config.yaml
```

修改以下配置：
- MySQL 连接信息
- Kafka 地址
- 腾讯云 API 密钥
- COS 桶信息

---

## 第二阶段：理解核心概念 (2-3小时)

### 2.1 先读这几篇文章/文档

建议按顺序阅读：

1. **README.md** - 整体概述（必读）
2. **message/message.go** - Kafka 消息格式
3. **pkg/constants/constants.go** - Topic 定义
4. **interface/interface.go** - 消费者接口

### 2.2 核心概念图解

```
                    ┌─────────────────┐
                    │   Web 浏览器    │
                    └────────┬────────┘
                             │ HTTP
                             ▼
                    ┌─────────────────┐
                    │ transform-web-api│ ← Web API服务
                    │   (Gin)         │
                    └────────┬────────┘
                             │ ExternalKafka
                             ▼
┌──────────────────────────────────────────────────────────────────┐
│                    transform 服务 (8个消费者)                    │
│                                                                   │
│   entry ──► av-extract ──► asr ──► refer-wav ──► translate     │
│                                                  │               │
│                              ◄────────────────────┘               │
│                                             │                     │
│                                    Kafka 内部流转                  │
│                                             │                     │
│   save-result ◄── av-synthesis ◄─ audio-generation                │
│                                                                   │
└──────────────────────────────────────────────────────────────────┘
                             │
                             │ 写入COS
                             ▼
                    ┌─────────────────┐
                    │   腾讯云 COS    │
                    │  (对象存储)     │
                    └─────────────────┘
```

### 2.3 关键数据结构

理解这三个结构就掌握了半个项目：

#### 2.3.1 Kafka 消息 (message/message.go)

```go
type KafkaMsg struct {
    RecordsID             int64   // 任务ID
    UserID                int64   // 用户ID
    OriginalVideoUrl     string  // 原始视频URL (COS)
    SourceLanguage       string  // 源语言 (如 "zh")
    TargetLanguage       string  // 目标语言 (如 "en")
    
    // 以下是各阶段填充的中间结果
    Filename             string  // 文件名
    ExtractVideoPath     string  // 提取的视频路径
    ExtractAudioPath     string  // 提取的音频路径
    OriginalSrtPath     string  // 原始字幕路径
    TranslateSrtPath    string  // 翻译字幕路径
    ReferWavPath        string  // 参考音频路径
    OutPutFilePath      string  // 最终输出路径
}
```

#### 2.3.2 消费者接口 (interface/interface.go)

```go
type ConsumerTask interface {
    Start(ctx context.Context)  // 启动消费者
}
```

每个处理模块都实现这个接口，监听特定的 Kafka topic。

#### 2.3.3 Topic 常量 (pkg/constants/constants.go)

```go
const (
    KAFKA_TOPIC_TRANSFORM_WEB_ENTRY       = "transform_web_entry"       // 入口
    KAFKA_TOPIC_TRANSFORM_AV_EXTRACT       = "transform_av_extract"     // 音视频提取
    KAFKA_TOPIC_TRANSFORM_ASR              = "transform_asr"            // 语音识别
    KAFKA_TOPIC_TRANSFORM_REFER_WAV        = "transform_refer_wav"      // 参考音频
    KAFKA_TOPIC_TRANSFORM_TRANSLATE_SRT    = "transform_translate_srt"  // 翻译
    KAFKA_TOPIC_TRANSFORM_AUDIO_GENERATION = "transform_audio_generation" // 音频生成
    KAFKA_TOPIC_TRANSFORM_AV_SYNTHESIS     = "transform_av_synthesis"   // 合成
    KAFKA_TOPIC_TRANSFORM_SAVE_RESULT       = "transform_save_result"    // 保存结果
)
```

---

## 第三阶段：代码阅读顺序 (3-4小时)

### 3.1 推荐阅读顺序

建议按以下顺序阅读源码：

#### Step 1: Web API (入口)

| 文件 | 重点 |
|------|------|
| `transform-web-api/main.go` | 服务启动 |
| `transform-web-api/controllers/transform.go` | 接收请求，发送到 Kafka |
| `transform-web-api/routers/routers.go` | 路由定义 |

#### Step 2: 消息定义

| 文件 | 重点 |
|------|------|
| `message/message.go` | 消息结构 |
| `pkg/constants/constants.go` | 常量定义 |

#### Step 3: 第一个消费者 (entry)

| 文件 | 重点 |
|------|------|
| `transform/entry/entry.go` | 从 Kafka 消费，下载视频 |
| 理解 `ConsumerTask` 接口 | 如何监听 Kafka |

#### Step 4: 核心处理模块

按顺序阅读，理解数据如何在各阶段流转：

1. `transform/av-extract/av_extract.go` - 音视频分离
2. `transform/asr/asr.go` - 语音识别
3. `transform/translate/translate.go` - 翻译
4. `transform/audio-generation/audio_generation.go` - 音频生成（重点）
5. `transform/av-synthesis/av_synthesis.go` - 音视频合成（重点）

#### Step 5: 基础设施

| 文件 | 重点 |
|------|------|
| `pkg/mq/kafka/consumer.go` | Kafka 消费者封装 |
| `pkg/storage/cos/cos.go` | COS 上传/下载 |
| `pkg/config/config.go` | 配置加载 |
| `pkg/go-pool/go_pool.go` | 并发控制 |

### 3.2 重点代码片段

#### Kafka 消费者模板

```go
// 每个处理模块的结构类似
type myModule struct {
    conf *config.Config
    log  log.ILogger
}

func NewMyModule(conf *config.Config, log log.ILogger) interface.ConsumerTask {
    return &myModule{conf: conf, log: log}
}

func (t *myModule) Start(ctx context.Context) {
    // 创建消费者配置
    conf := &kafka.ConsumerGroupConfig{...}
    // 创建消费者组
    cg := kafka.NewConsumerGroup(conf, t.log, t.messageHandleFunc)
    // 启动消费
    cg.Start(ctx, "topic-name", []string{"topic-name"})
}

func (t *myModule) messageHandleFunc(msg *sarama.ConsumerMessage) error {
    // 1. 解析消息
    data := &message.KafkaMsg{}
    json.Unmarshal(msg.Value, data)
    
    // 2. 业务处理
    // ...
    
    // 3. 发送到下一阶段
    producer := kafka.GetProducer(kafka.Producer)
    producer.SendMessage(&sarama.ProducerMessage{
        Topic: "next-topic",
        Value: sarama.StringEncoder(newValue),
    })
    return nil
}
```

---

## 第四阶段：调试运行 (2-3小时)

### 4.1 本地调试步骤

#### Step 1: 启动依赖服务

```bash
# 启动 MySQL
# 启动 Kafka (docker-compose)
# 确认 FFmpeg 可用
ffmpeg -version
```

#### Step 2: 修改配置

确保 `dev.config.yaml` 中的地址都是可访问的。

#### Step 3: 启动 transform 服务

```bash
cd ai-transform-backend
go run transform/main.go -config dev.config.yaml
```

观察日志，确认 8 个消费者都已启动：

```
[entry] Starting consumer...
[av-extract] Starting consumer...
[asr] Starting consumer...
...
```

#### Step 4: 启动 Web API

```bash
# 另一个终端
go run transform-web-api/main.go -config dev.config.yaml
```

#### Step 5: 发送测试请求

```bash
# 使用 curl 调用 API
curl -X POST http://localhost:8081/api/transform \
  -H "Content-Type: application/json" \
  -d '{
    "originalVideoUrl": "https://xxx.com/video.mp4",
    "sourceLanguage": "zh",
    "targetLanguage": "en"
  }'
```

观察 transform 服务的日志，可以看到消息在各阶段流转。

### 4.2 调试技巧

#### 查看 Kafka 消息

```bash
# 使用 kafka-console-consumer
docker exec -it <kafka-container> kafka-console-consumer \
  --bootstrap-server localhost:29092 \
  --topic transform_web_entry \
  --from-beginning
```

#### 查看中间产物

```bash
# 本地 runtime 目录
ls -la runtime/
# 查看各阶段产生的文件
ls -la runtime/middle/<filename>
ls -la runtime/srts/
```

---

## 第五阶段：深入理解 (持续)

### 5.1 需要掌握的重难点

按重要程度排序：

| 序号 | 难点 | 涉及文件 | 建议学习时长 |
|------|------|----------|--------------|
| 1 | 音视频同步 | av-synthesis | 2小时 |
| 2 | 并发控制 | audio-generation | 1小时 |
| 3 | ASR 过滤 | asr | 1小时 |
| 4 | 批量翻译 | translate | 1小时 |

### 5.2 推荐深入学习的代码

#### 5.2.1 音视频合成 (av-synthesis)

这是最复杂的模块，建议仔细阅读：

- `groupBySrt()` - 按时间分组
- `audioMerge()` - 音频合并
- `avMerge()` - 音视频合并
- `addSubtitles()` - 字幕烧录

#### 5.2.2 并发控制 (go-pool)

理解如何控制外部服务并发：

```go
// 创建协程池
pool := go_pool.NewPool(workerCount, executors...)
// 提交任务
pool.Schedule(task)
// 等待完成
pool.WaitAndClose()
```

#### 5.2.3 FFmpeg 调用

理解各种 FFmpeg 命令：

```bash
# 提取音频
ffmpeg -i input.mp4 -vn -acodec copy output.aac

# 提取视频
ffmpeg -i input.mp4 -an -vcodec copy output.mp4

# 音频延迟
ffmpeg -i input.wav -af "adelay=1000" output.wav

# 添加字幕
ffmpeg -i input.mp4 -vf "subtitles=xxx.srt" output.mp4

# 生成静音
ffmpeg -f lavfi -i anullsrc=r=44100:cl=mono -t 5000ms silence.wav
```

---

## 第六阶段：开发实践

### 6.1 添加新功能

假设要添加一个新阶段 `new-stage`：

1. **定义 Topic**
   在 `pkg/constants/constants.go` 添加：
   ```go
   KAFKA_TOPIC_TRANSFORM_NEW_STAGE = "transform_new_stage"
   ```

2. **创建处理模块**
   创建目录 `transform/new-stage/`，新建 `new_stage.go`：
   ```go
   package new_stage
   
   type newStage struct {...}
   
   func NewNewStage(...) interface.ConsumerTask {...}
   func (t *newStage) Start(ctx context.Context) {...}
   func (t *newStage) messageHandleFunc(...) error {...}
   ```

3. **注册到 main.go**
   在 `transform/main.go` 添加：
   ```go
   go new_stage.NewNewStage(cfg, logger).Start(ctx)
   ```

4. **修改上游**
   在上一个阶段的 `messageHandleFunc` 中发送到新 Topic。

### 6.2 常见问题排查

| 问题 | 可能原因 | 排查方法 |
|------|----------|----------|
| 消息卡在某阶段 | 消费者未启动/失败 | 检查日志 |
| ASR 识别失败 | 音频文件损坏/格式不支持 | 检查 FFmpeg 输出 |
| 翻译超时 | 网络问题/字符超限 | 检查批量逻辑 |
| 音频合成失败 | GPT-SoVITS 服务不可用 | 检查依赖服务 |

---

## 学习检查清单

完成以下任务，确保掌握项目：

- [ ] 本地搭建开发环境
- [ ] 理解整体架构和 8 个处理阶段
- [ ] 阅读 `message/message.go` 和 `constants.go`
- [ ] 阅读 `entry/entry.go` 理解消费者模式
- [ ] 阅读 `av-extract/av_extract.go` 理解 FFmpeg 调用
- [ ] 阅读 `av-synthesis/av_synthesis.go` 理解音视频合成
- [ ] 成功运行项目并完成一次完整翻译
- [ ] 能独立排查简单问题

---

## 参考资源

- [Go 语言中文文档](https://go.dev/doc/)
- [Kafka 官方文档](https://kafka.apache.org/documentation/)
- [FFmpeg 文档](https://ffmpeg.org/documentation.html)
- [腾讯云 ASR 文档](https://cloud.tencent.com/document/product/1093)
- [腾讯云 TMT 文档](https://cloud.tencent.com/document/product/551)
- [GPT-SoVITS 项目](https://github.com/GPT-SoVITS/GPT-SoVITS)

---

## 附录：快速命令参考

```bash
# 启动所有服务
go run transform/main.go -config dev.config.yaml
go run transform-web-api/main.go -config dev.config.yaml

# 查看 Kafka topic
kafka-topics.sh --list --bootstrap-server localhost:29092

# 查看消费者组
kafka-consumer-groups.sh --bootstrap-server localhost:29092 --list

# 查看实时日志
tail -f runtime/logs/app.log

# 测试 FFmpeg
ffmpeg -i input.mp4 -vn -acodec copy output.aac
```

---

*文档更新时间：2026-03-18*
