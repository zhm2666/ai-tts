# GPT-SoVITS 语音克隆项目分析文档

> 本文档基于 GPT-SoVITS-v2Pro-20250604-nvidia50 版本编写
> 分析日期: 2026-03-20

---

## 目录

1. [项目概述](#1-项目概述)
2. [项目结构](#2-项目结构)
3. [核心模块详解](#3-核心模块详解)
4. [语音克隆详细流程](#4-语音克隆详细流程)
5. [模型架构分析](#5-模型架构分析)
6. [训练流程](#6-训练流程)
7. [推理接口](#7-推理接口)
8. [版本差异](#8-版本差异)
9. [总结](#9-总结)

---

## 1. 项目概述

### 1.1 什么是 GPT-SoVITS

GPT-SoVITS 是一个强大的**少样本语音克隆**和**文本到语音 (TTS)** 项目，具有以下特点：

- **Zero-shot TTS**: 仅需 5 秒参考音频即可实现即时语音克隆
- **Few-shot TTS**: 1 分钟训练数据可显著提升音色相似度
- **跨语言支持**: 中文、英文、日文、韩语、粤语
- **WebUI 工具**: 集成人声分离、自动切分、中文 ASR 等预处理工具

### 1.2 技术架构

GPT-SoVITS 采用**双模型架构**：

| 模型 | 全称 | 作用 |
|------|------|------|
| **GPT** | Generative Pre-trained Transformer | 语义生成：根据文本和参考音色生成语义 Token 序列 |
| **SoVITS** | Sound Voice Intelligence Text to Speech | 声学解码：将语义 Token 转换为音频波形 |

---

## 2. 项目结构

### 2.1 整体目录

```
GPT-SoVITS-v2pro-20250604-nvidia50/
├── README.md                    # 项目主文档
├── config.py                    # 全局配置文件
├── requirements.txt             # Python 依赖
├── webui.py                     # 主 WebUI 入口
├── api.py                       # API 服务接口
├── GPT_SoVITS/                  # 核心模型代码
├── tools/                       # 预处理工具
└── logs/                        # 训练日志
```

### 2.2 GPT_SoVITS 核心目录

```
GPT_SoVITS/
├── AR/                          # GPT (自回归模型) 模块
│   ├── models/
│   │   ├── t2s_model.py         # Text-to-Semantic 模型 (核心)
│   │   ├── t2s_lightning_module.py  # Lightning 训练模块
│   │   └── t2s_model_onnx.py    # ONNX 导出
│   └── modules/
│       ├── transformer.py        # Transformer 架构
│       ├── embedding.py          # 嵌入层
│       └── lr_schedulers.py      # 学习率调度
│
├── module/                      # SoVITS (声码器) 模块
│   ├── models.py                 # 模型定义 (SynthesizerTrn)
│   ├── attentions.py             # 注意力机制
│   ├── commons.py                # 公共函数
│   ├── mel_processing.py         # Mel 频谱处理
│   ├── mrte_model.py             # MRTE 模块
│   └── quantize.py               # 量化器
│
├── text/                        # 文本处理模块
│   ├── cleaner.py                # 文本清洗与 G2P
│   ├── symbols.py                # 符号定义 (v1)
│   ├── symbols2.py               # 符号定义 (v2+)
│   ├── chinese.py                # 中文处理
│   ├── chinese2.py               # 中文处理 (v2)
│   ├── japanese.py               # 日文处理
│   ├── english.py                # 英文处理
│   ├── korean.py                 # 韩文处理
│   ├── cantonese.py              # 粤语处理
│   └── LangSegmenter/            # 语言分段器
│
├── feature_extractor/           # 特征提取
│   ├── cnhubert.py               # 中文 HuBERT 特征
│   └── whisper_enc.py            # Whisper 编码器
│
├── BigVGAN/                     # BigVGAN 声码器 (v3)
│   └── bigvgan.py
│
├── f5_tts/                      # F5-TTS 模型 (v3/v4)
│   └── model/backbones/dit.py    # Diffusion Transformer
│
├── eres2net/                    # ERES2Net 说话人验证 (v2Pro)
│   └── ERes2NetV2.py
│
├── pretrained_models/            # 预训练模型
│   ├── chinese-hubert-base/     # 中文 HuBERT
│   ├── chinese-roberta-wwm-ext-large/  # 中文 RoBERTa (BERT)
│   ├── gsv-v2final-pretrained/ # V2 预训练
│   ├── v2Pro/                   # V2Pro 预训练
│   └── sv/                      # 说话人验证模型
│
└── configs/                     # 配置文件
    ├── s1.yaml                  # GPT 训练配置
    └── s2.json                  # SoVITS 训练配置
```

### 2.3 tools 预处理工具目录

```
tools/
├── uvr5/                        # 人声伴奏分离
├── asr/                        # 语音识别
│   ├── funasr_asr.py           # FunASR (中文)
│   └── fasterwhisper_asr.py    # Faster Whisper (多语种)
├── slice_audio.py              # 音频切分
├── subfix_webui.py             # 文本标注
└── i18n/                       # 国际化
```

---

## 3. 核心模块详解

### 3.1 CNHuBERT 模块

**文件**: `GPT_SoVITS/feature_extractor/cnhubert.py`

CNHuBERT 是中文语音特征提取器，基于 Wav2Vec2 架构：

```python
class CNHubert(nn.Module):
    def __init__(self, base_path):
        self.model = HubertModel.from_pretrained(base_path)
        self.feature_extractor = Wav2Vec2FeatureExtractor.from_pretrained(base_path)

    def forward(self, x):
        input_values = self.feature_extractor(x, sampling_rate=16000)
        feats = self.model(input_values)["last_hidden_state"]
        return feats  # [batch, 768, time_steps]
```

**特点**:
- 输入: 16kHz 单声道音频
- 输出: 768 维 SSL 特征
- 时间分辨率: 50Hz (每20ms一帧)

### 3.2 VQ 量化器

**文件**: `GPT_SoVITS/module/quantize.py`

使用 Vector Quantization 将连续特征离散化：

```python
class ResidualVectorQuantizer:
    def __init__(self, dimension=768, n_q=1, bins=1024):
        # bins=1024 表示 10-bit 量化
        self.quantizer_modules = nn.ModuleList(
            [Quantizer_module(bins, dimension) for _ in range(n_q)]
        )
```

**作用**:
- 将 768 维 SSL 特征映射到 1024 个离散码本
- 每个量化值代表一个"语音基本单元"
- 量化后的 Token 序列包含说话人音色信息

### 3.3 TextEncoder 模块

**文件**: `GPT_SoVITS/module/models.py`

TextEncoder 负责将语义 Token 和文本信息融合：

```python
class TextEncoder(nn.Module):
    def __init__(self, ...):
        self.ssl_proj = nn.Conv1d(768, hidden_channels, 1)  # SSL 投影
        self.encoder_ssl = Encoder(...)  # SSL 编码器
        self.text_embedding = nn.Embedding(num_symbols, hidden_channels)  # 音素嵌入
        self.encoder_text = Encoder(...)  # 文本编码器
        self.mrte = MRTE()  # 多模态相对时间编码
        self.encoder2 = Encoder(...)  # 融合编码器
```

**关键组件**:

| 组件 | 作用 |
|------|------|
| `ssl_proj` | 将 768 维 SSL 投影到隐藏维度 |
| `encoder_ssl` | 编码 SSL 特征 |
| `text_embedding` | 音素的词嵌入表示 |
| `mrte` | 融合文本和音频信息 |
| `encoder2` | 进一步融合处理 |

### 3.4 Text2SemanticDecoder (GPT 模型)

**文件**: `GPT_SoVITS/AR/models/t2s_model.py`

GPT 模型负责根据文本和参考音色生成语义 Token：

```python
class Text2SemanticDecoder(nn.Module):
    def __init__(self, config):
        self.bert_proj = nn.Linear(1024, 512)      # BERT 特征投影
        self.ar_text_embedding = TokenEmbedding(512, phoneme_vocab_size)  # 音素嵌入
        self.ar_text_position = SinePositionalEmbedding(512)  # 位置编码
        self.ar_audio_embedding = TokenEmbedding(512, vocab_size)  # 语义 Token 嵌入
        self.h = TransformerEncoder(layers=12)     # 12 层 Transformer
        self.ar_predict_layer = nn.Linear(512, vocab_size)  # 预测层
```

**模型参数**:

| 参数 | 值 |
|------|-----|
| 隐藏维度 | 512 |
| 注意力头数 | 8 |
| 层数 | 12 |
| 音素词汇量 | 512 |
| 语义 Token 词汇量 | 1025 (1024 + EOS) |

### 3.5 Flow / CFM 模块

**文件**: `GPT_SoVITS/module/models.py`

v1/v2 使用 Flow 可逆变换，v3/v4 使用 CFM 扩散模型：

```python
# v1/v2: Flow
class ResidualCouplingBlock(nn.Module):
    def forward(self, x, x_mask, g=None, reverse=False):
        if not reverse:
            for flow in self.flows:
                x, _ = flow(x, x_mask, g=g, reverse=False)
        else:
            for flow in reversed(self.flows):
                x = flow(x, x_mask, g=g, reverse=True)
        return x

# v3/v4: CFM (Conditional Flow Matching)
class CFM(nn.Module):
    def inference(self, mu, x_lens, prompt, n_timesteps):
        x = torch.randn(...) * temperature
        for j in range(n_timesteps):
            v_pred = self.estimator(x, prompt, t_tensor, ...)
            x = x + d * v_pred
        return x
```

### 3.6 Vocoder 模块

**文件**: `GPT_SoVITS/module/models.py` / `GPT_SoVITS/BigVGAN/`

声码器负责将 Mel 频谱转换为波形：

| 版本 | 声码器 | 采样率 | 上采样率 |
|------|--------|--------|----------|
| v1/v2 | HiFi-GAN | 32kHz | x240 |
| v3 | BigVGAN | 24kHz | x188 |
| v4 | HiFi-GAN | 32kHz | x240 |

```python
class Generator(nn.Module):
    def __init__(self, ...):
        self.ups = nn.ModuleList([
            ConvTranspose1d(...),  # 上采样层
            ...
        ])
        self.resblocks = nn.ModuleList([...])  # ResBlock

    def forward(self, x, g=None):
        x = self.conv_pre(x)
        for i in range(self.num_upsamples):
            x = F.leaky_relu(x)
            x = self.ups[i](x)
            xs = sum([self.resblocks[i*...+j](x) for j in ...])
            x = xs / self.num_kernels
        x = torch.tanh(self.conv_post(x))
        return x
```

---

## 4. 语音克隆详细流程

### 4.1 整体流程图

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        GPT-SoVITS 语音克隆完整流程                             │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  【第一阶段】参考音频处理 ─── 提取说话人音色                                      │
│                                                                             │
│  参考音频(.wav) ──→ 16kHz重采样 ──→ CNHuBERT ──→ SSL特征(768维)               │
│                                                         │                   │
│                                                         ▼                   │
│                                                   VQ量化 ──→ 语义Token        │
│                                                               │              │
│  参考音频(.wav) ──→ 加载 ──→ 频谱分析 ──→ Mel频谱(100维)                       │
│                                                               │              │
│  【第二阶段】文本处理 ─── 理解要说什么                                                │
│                                                                             │
│  参考文本 ──→ 文本标准化 ──→ G2P ──→ 音素序列                                  │
│                                                         │                   │
│                                                         ▼                   │
│                                                   BERT ──→ 语义特征(1024维)     │
│                                                                             │
│  目标文本 ──→ 文本标准化 ──→ G2P ──→ 音素序列                                  │
│                                                         │                   │
│                                                         ▼                   │
│                                                   BERT ──→ 语义特征(1024维)     │
│                                                                             │
│  【第三阶段】GPT生成 ─── 决定"用这个音色说什么"                                   │
│                                                                             │
│  输入:                                                                    │
│    - 参考文本音素 + BERT                                                      │
│    - 目标文本音素 + BERT                                                      │
│    - 参考音频语义Token (prompt)                                              │
│                                                                             │
│  自回归生成:                                                                 │
│    prompt = [t0, t1, t2, ...]  ← 参考音频的语义Token                          │
│    输入 = [音素序列] + [t0, t1, t2, ...]                                    │
│    输出 = t_next (下一个语义Token)                                          │
│                                                                             │
│  输出: 生成的语义Token序列 (pred_semantic)                                   │
│                                                                             │
│  【第四阶段】SoVITS解码 ─── 转换为音频                                         │
│                                                                             │
│  语义Token ──→ VQ解码 ──→ 中间特征                                           │
│                                    │                                        │
│  参考Mel ───────────────────────────┘                                        │
│                                    │                                        │
│                                    ▼                                        │
│                           TextEncoder ──→ 融合文本+音色                      │
│                                    │                                        │
│                                    ▼                                        │
│                           Flow/CFM ──→ 潜在变量变换                          │
│                                    │                                        │
│                                    ▼                                        │
│                           Mel频谱 ──→ Vocoder ──→ 音频波形                   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 4.2 分步详细说明

#### 第一步：参考音频处理

**1.1 音频加载与预处理**

```python
# api.py 第871-877行
wav16k, sr = librosa.load(ref_wav_path, sr=16000)  # 重采样到16kHz
wav16k = torch.from_numpy(wav16k)                   # 转为张量
wav16k = torch.cat([wav16k, zero_wav_torch])       # 拼接0.3秒静音
```

**参数**:
- 采样率: 16kHz (CNHuBERT 标准输入)
- 格式: 单声道浮点
- 拼接静音: 防止卷积边缘效应

**1.2 SSL 特征提取**

```python
# api.py 第882行
ssl_content = ssl_model.model(wav16k.unsqueeze(0))["last_hidden_state"]
# 输出: [batch, 768, time_steps]
```

CNHuBERT 模型架构:
- 基于 Wav2Vec2
- 预训练于大量中文语音
- 输出 768 维 SSL 特征

**1.3 VQ 量化**

```python
# api.py 第883-885行
codes = vq_model.extract_latent(ssl_content)
prompt_semantic = codes[0, 0]  # 第一个样本，第一个通道
prompt = prompt_semantic.unsqueeze(0)  # [1, prompt_len]
```

量化过程:
```
SSL特征 (768维) ──→ Conv1d投影 ──→ 最近邻码本查找 ──→ 语义Token
                                          ↑
                                     码本大小: 1024
```

**1.4 提取参考音频 Mel 频谱**

```python
# api.py 第897-909行
refer, audio_tensor = get_spepc(hps, ref_wav_path, dtype, device)
ref_audio, sr = torchaudio.load(ref_wav_path)
mel2 = mel_fn(ref_audio)  # 提取Mel频谱
mel2 = norm_spec(mel2)     # 归一化
```

Mel 频谱参数:

| 版本 | 采样率 | n_fft | hop_size | mel_bins |
|------|--------|-------|----------|----------|
| v1/v2 | 32kHz | 1024 | 256 | 100 |
| v3 | 24kHz | 1280 | 320 | 100 |
| v4 | 32kHz | 1280 | 320 | 100 |

#### 第二步：文本处理

**2.1 文本标准化与 G2P**

```python
# cleaner.py 第21-55行
def clean_text(text, language, version=None):
    # 1. 语言特定标准化
    norm_text = language_module.text_normalize(text)

    # 2. G2P 转换
    if language == "zh" or language == "yue":
        phones, word2ph = language_module.g2p(norm_text)
    else:
        phones = language_module.g2p(norm_text)

    # 3. 映射到符号表
    phones = ["UNK" if ph not in symbols else ph for ph in phones]
    return phones, word2ph, norm_text
```

**处理示例**:

```
原始文本: "你好，世界！"
    ↓
文本标准化: "你好，世界！" (数字转汉字等)
    ↓
G2P转换: ["n", "i", "h", "ao", " ", "sh", "i4", "j", "ie4", ...]
    ↓
映射: [音素索引, ...]
```

**2.2 BERT 语义特征提取**

```python
# api.py 第505-519行
def get_bert_feature(text, word2ph):
    inputs = tokenizer(text, return_tensors="pt")
    res = bert_model(**inputs, output_hidden_states=True)
    # 选择倒数第3层 (经验最佳)
    res = torch.cat(res["hidden_states"][-3:-2], -1)[0][1:-1]

    # 音素级别对齐
    phone_level_feature = []
    for i in range(len(word2ph)):
        repeat_feature = res[i].repeat(word2ph[i], 1)
        phone_level_feature.append(repeat_feature)
    return torch.cat(phone_level_feature, dim=0).T
```

BERT 特征维度:
- 输入: 文本序列
- 输出: 1024 维
- 与音素对齐后: [1024, phone_count]

#### 第三步：GPT 模型生成

**3.1 自回归生成**

```python
# t2s_model.py 第814-960行
def infer_panel_naive(self, x, x_lens, prompts, bert_feature, ...):
    # 1. 准备输入
    x = self.ar_text_embedding(x)
    x = x + self.bert_proj(bert_feature.transpose(1, 2))
    x = self.ar_text_position(x)

    # 2. 准备 prompt (参考音频的语义 Token)
    y = prompts  # [batch, prompt_len]

    # 3. 自回归生成 (最多1500步)
    for idx in range(1500):
        # 处理 prompt
        xy_dec, k_cache, v_cache = self.t2s_transformer.process_prompt(...)

        # 自回归解码
        xy_dec, k_cache, v_cache = self.t2s_transformer.decode_next_token(...)

        # 预测下一个 token
        logits = self.ar_predict_layer(xy_dec[:, -1])

        # 采样
        samples = sample(logits, y, top_k, top_p, temperature)

        # 拼接
        y = torch.concat([y, samples], dim=1)

        # 检查停止条件
        if EOS_generated or max_length_reached:
            break

    return y, idx
```

**自回归生成图解**:

```
时间步 0:
  prompt = [t₀, t₁, t₂, t₃]  (参考音频的语义Token)
  输入   = [音素₁, 音素₂, ..., 音素ₙ] + [t₀, t₁, t₂, t₃]
  输出   = t₄  (预测的下一个语义Token)

时间步 1:
  prompt = [t₀, t₁, t₂, t₃, t₄]
  输入   = [音素₁, 音素₂, ..., 音素ₙ] + [t₀, t₁, t₂, t₃, t₄]
  输出   = t₅

... 继续直到遇到 EOS
```

**生成控制参数**:

```python
top_k = 15           # Top-K 采样，保留概率最高的15个token
top_p = 0.6          # Nucleus采样，累积概率达到0.6
temperature = 0.6    # 温度参数，控制随机性
early_stop_num = 50  # 最多生成50个token（约1秒语音）
```

#### 第四步：SoVITS 解码

**4.1 v1/v2 版本解码**

```python
# models.py 第994-1039行
@torch.no_grad()
def decode(self, codes, text, refer, noise_scale=0.5, speed=1, sv_emb=None):
    # 1. 获取说话人嵌入
    ge = self.ref_enc(refer)  # 从参考音频提取
    if self.is_v2pro:
        sv_emb = self.sv_emb(sv_emb)  # 加入 SV embedding
        ge += sv_emb

    # 2. 语义 Token 解码
    quantized = self.quantizer.decode(codes)  # [batch, 768, time]

    # 3. TextEncoder 融合
    x, m_p, logs_p, y_mask = self.enc_p(
        quantized, y_lengths, text, text_lengths, ge, speed
    )

    # 4. Flow 变换
    z_p = m_p + torch.randn_like(m_p) * torch.exp(logs_p) * noise_scale
    z = self.flow(z_p, y_mask, g=ge, reverse=True)

    # 5. HiFi-GAN 生成波形
    o = self.dec(z * y_mask, g=ge)
    return o
```

**4.2 v3/v4 版本解码 (CFM)**

```python
# models.py 第1100-1172行
class CFM(nn.Module):
    @torch.inference_mode()
    def inference(self, mu, x_lens, prompt, n_timesteps, ...):
        # 初始化噪声
        x = torch.randn([B, channels, T], device=mu.device) * temperature

        # 扩散采样
        for j in range(n_timesteps):
            v_pred = self.estimator(x, prompt, t_tensor, ...)
            x = x + d * v_pred  # 迭代更新

        return x
```

#### 第五步：后处理

```python
# api.py 第1027-1068行
# 1. 归一化
max_audio = np.abs(audio).max()
if max_audio > 1:
    audio /= max_audio

# 2. 拼接静音
audio_opt.append(audio)
audio_opt.append(zero_wav)
audio_opt = np.concatenate(audio_opt, 0)

# 3. 编码打包
if is_int32:
    audio_bytes = pack_audio((audio_opt * 2147483647).astype(np.int32), sr)
else:
    audio_bytes = pack_audio((audio_opt * 32768).astype(np.int16), sr)

# 4. 返回
return audio_bytes
```

---

## 5. 模型架构分析

### 5.1 数据流汇总

| 阶段 | 数据 | 维度 | 说明 |
|------|------|------|------|
| 输入 | 参考音频 | [samples] | 16kHz 单声道 |
| 1.1 | 波形张量 | [1, samples] | PyTorch 张量 |
| 1.2 | SSL 特征 | [batch, 768, T] | CNHuBERT 输出 |
| 1.3 | 语义 Token | [1, prompt_len] | VQ 量化后 |
| 1.4 | Mel 频谱 | [batch, 100, T] | 参考音频 |
| 2.1 | 音素序列 | [phone_count] | 文本转音素 |
| 2.2 | BERT 特征 | [1024, phone_count] | 语义表示 |
| 3 | 生成语义 | [1, gen_len] | GPT 自回归 |
| 4.1 | 中间特征 | [batch, 512, T] | TextEncoder |
| 4.2 | Mel 频谱 | [batch, 100, T] | Flow 输出 |
| 4.3 | 波形 | [samples] | Vocoder 输出 |
| 5 | 编码音频 | bytes | wav/ogg/aac |

### 5.2 核心创新点

#### 创新点 1: 双阶段分离设计

```
GPT 模型 (语义生成)     SoVITS 模型 (声学解码)
┌─────────────────┐    ┌─────────────────┐
│                 │    │                 │
│  文本 + 音色    │    │  语义 Token    │
│       ↓         │    │       ↓         │
│  语义 Token     │    │  音频波形      │
│                 │    │                 │
└─────────────────┘    └─────────────────┘
```

**优势**:
- 语义生成和声学生成解耦
- 可以独立优化
- 便于迁移学习

#### 创新点 2: 语义 Token 的桥梁作用

```
参考音频 ──[CNHuBERT]──→ SSL特征 ──[VQ]──→ 语义Token ──[GPT]──→ 新语义Token ──[SoVITS]──→ 音频
              ↑                                              ↑
           音色信息                                        音色信息保留
```

**关键洞察**:
- VQ 量化后的 Token 序列保留了说话人音色信息
- GPT 模型通过 Attention 机制学习音色模式
- 生成的 Token 序列携带目标音色

#### 创新点 3: 轻量级语言克隆

| 方法 | 参考音频需求 | 训练需求 |
|------|-------------|----------|
| 传统 TTS | 30分钟+ | 多小时 |
| YourTTS | 1分钟+ | 数小时 |
| **GPT-SoVITS** | **5秒-1分钟** | **数分钟-数小时** |

---

## 6. 训练流程

### 6.1 完整训练流程

```
原始音频
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ 0. 前置处理                                                    │
│   ├── 0a: UVR5 人声伴奏分离 & 去混响                           │
│   ├── 0b: 音频自动切分                                        │
│   ├── 0c: ASR 语音识别 (FunASR / Whisper)                     │
│   └── 0d: 文本标注校对                                        │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ 1. 数据集格式化 (一键三连)                                     │
│   ├── 1Aa: 文本分词 + BERT 特征提取                           │
│   ├── 1Ab: HuBERT 特征提取 (16kHz)                           │
│   └── 1Ac: 语义 Token 提取 (VQ)                              │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ 2. 模型训练                                                   │
│   ├── 2a: SoVITS (声码器) 训练                               │
│   │     - SSL 特征提取                                       │
│   │     - VQ 量化                                            │
│   │     - TextEncoder + Flow + HiFi-GAN                     │
│   │     - 损失: 重建损失 + VQ 损失                           │
│   │                                                         │
│   └── 2b: GPT (语言模型) 训练                                │
│         - 语义 Token 预测                                     │
│         - 交叉熵损失                                         │
│         - 可选: DPO 对比学习                                  │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ 3. 推理                                                       │
│   └── 3a: TTS 推理 WebUI / API                               │
└─────────────────────────────────────────────────────────────┘
```

### 6.2 预处理工具

| 工具 | 用途 | 使用场景 |
|------|------|----------|
| UVR5 | 人声伴奏分离 | 移除背景音乐 |
| slice_audio | 音频切分 | 切分长音频为短句 |
| funasr_asr | 中文 ASR | 自动生成文本标注 |
| fasterwhisper_asr | 多语种 ASR | 英文/日文等标注 |

---

## 7. 推理接口

### 7.1 API 调用方式

```python
# API 服务启动
python api.py -dr "ref.wav" -dt "参考文本" -dl "zh" -s "SoVITS模型.pth" -g "GPT模型.ckpt"

# GET 请求
GET http://127.0.0.1:9880?refer_wav_path=123.wav&prompt_text=一二三。&prompt_language=zh&text=先帝创业未半&text_language=zh

# POST 请求
POST http://127.0.0.1:9880/
{
    "refer_wav_path": "123.wav",
    "prompt_text": "一二三。",
    "prompt_language": "zh",
    "text": "先帝创业未半而中道崩殂",
    "text_language": "zh",
    "top_k": 15,
    "top_p": 0.6,
    "temperature": 0.6,
    "speed": 1.0
}
```

### 7.2 参数说明

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| refer_wav_path | string | 必填 | 参考音频路径 |
| prompt_text | string | 必填 | 参考音频文本 |
| prompt_language | string | 必填 | 参考音频语种 |
| text | string | 必填 | 目标文本 |
| text_language | string | 必填 | 目标文本语种 |
| top_k | int | 15 | Top-K 采样 |
| top_p | float | 1.0 | Nucleus 采样 |
| temperature | float | 1.0 | 温度参数 |
| speed | float | 1.0 | 语速 |
| sample_steps | int | 32 | 采样步数 (v3/v4) |
| inp_refs | list | [] | 额外参考音频 |
| cut_punc | string | "" | 断句符号 |

### 7.3 支持的语言

```python
dict_language = {
    "中文": "all_zh",
    "粤语": "all_yue",
    "英文": "en",
    "日文": "all_ja",
    "韩文": "all_ko",
    "中英混合": "zh",
    "粤英混合": "yue",
    "日英混合": "ja",
    "韩英混合": "ko",
    "多语种混合": "auto",
    "多语种混合(粤语)": "auto_yue",
}
```

---

## 8. 版本差异

### 8.1 各版本特性

| 版本 | SoVITS 底模 | GPT 底模 | 采样率 | 特点 |
|------|-------------|----------|--------|------|
| v1 | s2G488k.pth | s1bert25hz | 32kHz | 基础版本 |
| v2 | s2G2333k.pth | s1bert25hz-5kh | 32kHz | 支持韩粤 |
| v3 | s2Gv3.pth | s1v3.ckpt | 24kHz | BigVGAN, CFM |
| v4 | s2Gv4.pth | s1v3.ckpt | 32kHz | HiFi-GAN |
| v2Pro | s2Gv2Pro.pth | s1v3.ckpt | 32kHz | +SV embedding |
| v2ProPlus | s2Gv2ProPlus.pth | s1v3.ckpt | 32kHz | 最高质量 |

### 8.2 技术差异

| 特性 | v1/v2 | v3/v4 | v2Pro |
|------|-------|-------|-------|
| 声码器 | HiFi-GAN | BigVGAN/HiFi-GAN | HiFi-GAN |
| 潜在变换 | Flow | CFM (扩散) | Flow |
| 说话人编码 | Mel-Style | Mel-Style | SV Embedding |
| 采样步数 | 1 | 4-128 | 1 |
| 流式推理 | 支持 | 不支持 | 支持 |

---

## 9. 总结

### 9.1 核心技术要点

1. **双模型架构**: GPT 负责语义生成，SoVITS 负责声学合成
2. **语义 Token**: 通过 VQ 量化将语音离散化，作为模型间传递信息的桥梁
3. **轻量克隆**: 5秒-1分钟参考音频即可实现语音克隆
4. **多语言支持**: 内置多语言 G2P 和 BERT 模型

### 9.2 关键技术组件

| 组件 | 作用 | 文件 |
|------|------|------|
| CNHuBERT | SSL 特征提取 | cnhubert.py |
| VQ Quantizer | 语义离散化 | quantize.py |
| TextEncoder | 文本-语义融合 | models.py |
| Text2SemanticDecoder | GPT 生成 | t2s_model.py |
| Flow/CFM | 潜在变换 | models.py |
| HiFi-GAN/BigVGAN | 波形生成 | models.py, bigvgan.py |

### 9.3 使用建议

- **Zero-shot 场景**: 使用 v2Pro 版本，无需训练
- **Few-shot 场景**: 使用 v2 或 v3，1分钟数据微调
- **高质量需求**: 使用 v2ProPlus，预训练底模 + LoRA

---

## 参考资料

- [GPT-SoVITS GitHub](https://github.com/RVC-Boss/GPT-SoVITS)
- [VALL-E 论文](https://arxiv.org/abs/2301.02111)
- [SoundStorm 论文](https://arxiv.org/abs/2307.15272)
- [HiFi-GAN 论文](https://arxiv.org/abs/2010.05646)
- [BigVGAN 论文](https://nvidia.github.io/BigVGAN/)

---

*文档生成时间: 2026-03-20*
