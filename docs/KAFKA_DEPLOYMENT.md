# AI-Transform Kafka 部署指南

## 1. 环境要求

### 1.1 硬件要求

| 组件 | 最低配置 | 推荐配置 |
|------|----------|----------|
| CPU | 2核 | 4核 |
| 内存 | 4GB | 8GB |
| 磁盘 | 50GB SSD | 100GB+ SSD |
| 网络 | 100Mbps | 1Gbps |

### 1.2 软件要求

- **操作系统**: CentOS 7+ / Ubuntu 20.04+ / Rocky Linux 8+
- **Docker**: 20.10+
- **Docker Compose**: 2.0+

### 1.3 网络规划

本项目需要部署 **两个 Kafka 集群**：

| 集群 | 类型 | 节点数 | 端口 | 用途 |
|------|------|--------|------|------|
| **ExternalKafka** | 单节点 | 1 | 59092 | Web API 入口 |
| **InternalKafka** | 集群 | 3 | 29092/39092/49092 | 内部处理 |

---

## 2. 前置准备

### 2.1 安装 Docker

```bash
# CentOS/Rocky Linux
sudo yum install -y yum-utils
sudo yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
sudo yum install docker-ce docker-ce-cli containerd.io
sudo systemctl start docker
sudo systemctl enable docker

# Ubuntu
sudo apt update
sudo apt install docker.io docker-compose
sudo systemctl start docker
sudo systemctl enable docker
```

### 2.2 验证 Docker

```bash
docker --version
docker-compose --version
```

---

## 3. 创建配置文件目录

```bash
# 创建配置目录
mkdir -p /opt/kafka-config/external
mkdir -p /opt/kafka-config/internal/{kafka1,kafka2,kafka3}

# 修改权限
chmod -R 755 /opt/kafka-config
```

---

## 4. ExternalKafka 部署（单节点）

### 4.1 创建 JAAS 配置文件

```bash
cat > /opt/kafka-config/external/kafka_server_jaas.conf << 'EOF'
KafkaServer {
org.apache.kafka.common.security.plain.PlainLoginModule required
username="admin"
password="123456"
user_admin="123456";
};
EOF
```

### 4.2 创建 server.properties

```bash
cat > /opt/kafka-config/external/server.properties << 'EOF'
# ==================== 基础配置 ====================
process.roles=broker,controller
node.id=1
controller.quorum.voters=1@kafka-single:9093

# ==================== 监听器配置 ====================
listeners=SASL_PLAINTEXT://:9092,CONTROLLER://kafka-single:9093,INTER_BROKER://kafka-single:19092
inter.broker.listener.name=INTER_BROKER
# 注意：192.168.239.164 改为你的服务器IP
advertised.listeners=INTER_BROKER://kafka-single:19092,SASL_PLAINTEXT://192.168.239.164:59092
controller.listener.names=CONTROLLER

listener.security.protocol.map=CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT,SSL:SSL,SASL_PLAINTEXT:SASL_PLAINTEXT,SASL_SSL:SASL_SSL,INTER_BROKER:SASL_PLAINTEXT

# ==================== 安全认证 ====================
sasl.mechanism.inter.broker.protocol=PLAIN
sasl.enabled.mechanisms=PLAIN

# ==================== 线程配置 ====================
num.network.threads=3
num.io.threads=8
socket.send.buffer.bytes=102400
socket.receive.buffer.bytes=102400
socket.request.max.bytes=104857600

# ==================== 日志存储 ====================
log.dirs=/data/kafka/logs
num.partitions=1
num.recovery.threads.per.data.dir=1

# ==================== 副本配置 ====================
offsets.topic.replication.factor=1
transaction.state.log.replication.factor=1
transaction.state.log.min.isr=1

# ==================== 保留策略 ====================
log.retention.hours=168
log.segment.bytes=1073741824
log.retention.check.interval.ms=300000
EOF
```

### 4.3 启动 ExternalKafka

```bash
# 创建 Docker 网络
docker network create kafka-net

# 创建数据卷
docker volume create kafka-external-data

# 设置权限
docker run --rm \
  -v kafka-external-data:/data/kafka/logs \
  quay.io/0voice/alpine:3.18 \
  sh -c "chown -R 1000:1000 /data/kafka/logs && chmod -R 755 /data/kafka/logs"

# 生成集群ID
KAFKA_CLUSTER_ID=$(docker run --rm confluentinc/cp-kafka:7.5.0 kafka-broker-api-versions --bootstrap-server localhost:9092 2>/dev/null || echo "not_ready")
# 使用 uuidgen 生成
KAFKA_CLUSTER_ID=$(uuidgen)
echo "Cluster ID: $KAFKA_CLUSTER_ID"

# 启动容器
docker run -d \
  --name kafka-external \
  --network kafka-net \
  -p 59092:9092 \
  -e CLUSTER_ID=$KAFKA_CLUSTER_ID \
  -e KAFKA_OPTS="-Djava.security.auth.login.config=/mnt/shared/config/kafka_server_jaas.conf" \
  -v kafka-external-data:/data/kafka/logs \
  -v /opt/kafka-config/external:/mnt/shared/config \
  apache/kafka:3.7.0
```

### 4.4 验证 ExternalKafka

```bash
# 进入容器
docker exec -it kafka-external bash

# 创建客户端配置文件（在容器内执行）
cat > /opt/kafka-config/client.properties << 'EOF'
security.protocol=SASL_PLAINTEXT
sasl.mechanism=PLAIN
sasl.jaas.config=org.apache.kafka.common.security.plain.PlainLoginModule required username="admin" password="123456";
EOF

# 测试连接
/opt/kafka/bin/kafka-broker-api-versions.sh --bootstrap-server 192.168.239.164:59092 --command-config /opt/kafka-config/client.properties
```

---

## 5. InternalKafka 部署（3节点集群）

### 5.1 创建 JAAS 配置文件

```bash
cat > /opt/kafka-config/internal/kafka_server_jaas.conf << 'EOF'
KafkaServer {
org.apache.kafka.common.security.plain.PlainLoginModule required
username="admin"
password="123456"
user_admin="123456";
};
EOF
```

### 5.2 创建节点1配置

```bash
cat > /opt/kafka-config/internal/kafka1/server.properties << 'EOF'
process.roles=broker,controller
node.id=1
controller.quorum.voters=1@kafka-1:9093,2@kafka-2:9093,3@kafka-3:9093

listeners=SASL_PLAINTEXT://:9092,CONTROLLER://kafka-1:9093,INTER_BROKER://kafka-1:19092
inter.broker.listener.name=INTER_BROKER
advertised.listeners=INTER_BROKER://kafka-1:19092,SASL_PLAINTEXT://192.168.239.164:29092
controller.listener.names=CONTROLLER

listener.security.protocol.map=CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT,SSL:SSL,SASL_PLAINTEXT:SASL_PLAINTEXT,SASL_SSL:SASL_SSL,INTER_BROKER:SASL_PLAINTEXT

sasl.mechanism.inter.broker.protocol=PLAIN
sasl.enabled.mechanisms=PLAIN

num.network.threads=3
num.io.threads=8
socket.send.buffer.bytes=102400
socket.receive.buffer.bytes=102400
socket.request.max.bytes=104857600

log.dirs=/data/kafka/logs
num.partitions=3
num.recovery.threads.per.data.dir=1

offsets.topic.replication.factor=3
transaction.state.log.replication.factor=3
transaction.state.log.min.isr=2

log.retention.hours=168
log.segment.bytes=1073741824
log.retention.check.interval.ms=300000
EOF
```

### 5.3 创建节点2配置

```bash
cat > /opt/kafka-config/internal/kafka2/server.properties << 'EOF'
process.roles=broker,controller
node.id=2
controller.quorum.voters=1@kafka-1:9093,2@kafka-2:9093,3@kafka-3:9093

listeners=SASL_PLAINTEXT://:9092,CONTROLLER://kafka-2:9093,INTER_BROKER://kafka-2:19092
inter.broker.listener.name=INTER_BROKER
advertised.listeners=INTER_BROKER://kafka-2:19092,SASL_PLAINTEXT://192.168.239.164:39092
controller.listener.names=CONTROLLER

listener.security.protocol.map=CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT,SSL:SSL,SASL_PLAINTEXT:SASL_PLAINTEXT,SASL_SSL:SASL_SSL,INTER_BROKER:SASL_PLAINTEXT

sasl.mechanism.inter.broker.protocol=PLAIN
sasl.enabled.mechanisms=PLAIN

num.network.threads=3
num.io.threads=8
socket.send.buffer.bytes=102400
socket.receive.buffer.bytes=102400
socket.request.max.bytes=104857600

log.dirs=/data/kafka/logs
num.partitions=3
num.recovery.threads.per.data.dir=1

offsets.topic.replication.factor=3
transaction.state.log.replication.factor=3
transaction.state.log.min.isr=2

log.retention.hours=168
log.segment.bytes=1073741824
log.retention.check.interval.ms=300000
EOF
```

### 5.4 创建节点3配置

```bash
cat > /opt/kafka-config/internal/kafka3/server.properties << 'EOF'
process.roles=broker,controller
node.id=3
controller.quorum.voters=1@kafka-1:9093,2@kafka-2:9093,3@kafka-3:9093

listeners=SASL_PLAINTEXT://:9092,CONTROLLER://kafka-3:9093,INTER_BROKER://kafka-3:19092
inter.broker.listener.name=INTER_BROKER
advertised.listeners=INTER_BROKER://kafka-3:19092,SASL_PLAINTEXT://192.168.239.164:49092
controller.listener.names=CONTROLLER

listener.security.protocol.map=CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT,SSL:SSL,SASL_PLAINTEXT:SASL_PLAINTEXT,SASL_SSL:SASL_SSL,INTER_BROKER:SASL_PLAINTEXT

sasl.mechanism.inter.broker.protocol=PLAIN
sasl.enabled.mechanisms=PLAIN

num.network.threads=3
num.io.threads=8
socket.send.buffer.bytes=102400
socket.receive.buffer.bytes=102400
socket.request.max.bytes=104857600

log.dirs=/data/kafka/logs
num.partitions=3
num.recovery.threads.per.data.dir=1

offsets.topic.replication.factor=3
transaction.state.log.replication.factor=3
transaction.state.log.min.isr=2

log.retention.hours=168
log.segment.bytes=1073741824
log.retention.check.interval.ms=300000
EOF
```

### 5.5 启动 InternalKafka 集群

```bash
# 创建数据卷
docker volume create kafka-internal-1-data
docker volume create kafka-internal-2-data
docker volume create kafka-internal-3-data

# 设置权限
for i in 1 2 3; do
  docker run --rm \
    -v kafka-internal-$i-data:/data/kafka/logs \
    quay.io/0voice/alpine:3.18 \
    sh -c "chown -R 1000:1000 /data/kafka/logs && chmod -R 755 /data/kafka/logs"
done

# 生成集群ID（三个节点使用同一个ID）
KAFKA_CLUSTER_ID=$(uuidgen)
echo "Cluster ID: $KAFKA_CLUSTER_ID"

# 启动节点1
docker run -d \
  --name kafka-internal-1 \
  --network kafka-net \
  -p 29092:9092 \
  -e CLUSTER_ID=$KAFKA_CLUSTER_ID \
  -e KAFKA_OPTS="-Djava.security.auth.login.config=/mnt/shared/config/kafka_server_jaas.conf" \
  -v kafka-internal-1-data:/data/kafka/logs \
  -v /opt/kafka-config/internal/kafka1:/mnt/shared/config \
  apache/kafka:3.7.0

# 启动节点2
docker run -d \
  --name kafka-internal-2 \
  --network kafka-net \
  -p 39092:9092 \
  -e CLUSTER_ID=$KAFKA_CLUSTER_ID \
  -e KAFKA_OPTS="-Djava.security.auth.login.config=/mnt/shared/config/kafka_server_jaas.conf" \
  -v kafka-internal-2-data:/data/kafka/logs \
  -v /opt/kafka-config/internal/kafka2:/mnt/shared/config \
  apache/kafka:3.7.0

# 启动节点3
docker run -d \
  --name kafka-internal-3 \
  --network kafka-net \
  -p 49092:9092 \
  -e CLUSTER_ID=$KAFKA_CLUSTER_ID \
  -e KAFKA_OPTS="-Djava.security.auth.login.config=/mnt/shared/config/kafka_server_jaas.conf" \
  -v kafka-internal-3-data:/data/kafka/logs \
  -v /opt/kafka-config/internal/kafka3:/mnt/shared/config \
  apache/kafka:3.7.0
```

### 5.6 验证 InternalKafka 集群

```bash
# 进入容器
docker exec -it kafka-internal-1 bash

# 创建客户端配置文件（在容器内执行）
cat > /opt/kafka/config/client.properties << 'EOF'
security.protocol=SASL_PLAINTEXT
sasl.mechanism=PLAIN
sasl.jaas.config=org.apache.kafka.common.security.plain.PlainLoginModule required username="admin" password="123456";
EOF

# 测试连接（任一节点）
/opt/kafka/bin/kafka-broker-api-versions.sh --bootstrap-server 192.168.239.164:29092 --command-config /opt/kafka/config/client.properties

# 查看所有 Topic
/opt/kafka/bin/kafka-topics.sh --bootstrap-server 192.168.239.164:29092 --list --command-config /opt/kafka/config/client.properties
```

---

## 6. 创建 Kafka Topics

### 6.1 进入 Kafka 容器

```bash
docker exec -it kafka-internal-1 bash
```

### 6.2 创建 Topics（8个处理阶段）

```bash
# 创建客户端配置
cat > /opt/kafka/config/client.properties << 'EOF'
security.protocol=SASL_PLAINTEXT
sasl.mechanism=PLAIN
sasl.jaas.config=org.apache.kafka.common.security.plain.PlainLoginModule required username="admin" password="123456";
EOF

# 创建入口Topic
/opt/kafka/bin/kafka-topics.sh --create \
  --bootstrap-server 192.168.239.164:29092 \
  --replication-factor 1 \
  --partitions 3 \
  --topic transform_web_entry \
  --command-config /opt/kafka/config/client.properties

# 创建内部处理Topics
for topic in transform_av_extract transform_asr transform_refer_wav transform_translate_srt transform_audio_generation transform_av_synthesis transform_save_result; do
  /opt/kafka/bin/kafka-topics.sh --create \
    --bootstrap-server 192.168.239.164:29092 \
    --replication-factor 3 \
    --partitions 3 \
    --topic $topic \
    --command-config /opt/kafka/config/client.properties
done

# 验证 Topics 创建
/opt/kafka/bin/kafka-topics.sh --bootstrap-server 192.168.239.164:29092 --list --command-config /opt/kafka/config/client.properties
```

---

## 7. Docker Compose 部署（推荐）

### 7.1 创建 docker-compose.yml

```bash
cat > /opt/kafka-config/docker-compose.yml << 'EOF'
version: '3.8'

services:
  # ExternalKafka - 单节点
  kafka-external:
    image: apache/kafka:3.7.0
    container_name: kafka-external
    network_mode: kafka-net
    ports:
      - "59092:9092"
    environment:
      CLUSTER_ID: ${KAFKA_EXTERNAL_CLUSTER_ID}
      KAFKA_OPTS: "-Djava.security.auth.login.config=/mnt/shared/config/kafka_server_jaas.conf"
    volumes:
      - kafka-external-data:/data/kafka/logs
      - ./external:/mnt/shared/config
    restart: unless-stopped

  # InternalKafka - 节点1
  kafka-internal-1:
    image: apache/kafka:3.7.0
    container_name: kafka-internal-1
    network_mode: kafka-net
    ports:
      - "29092:9092"
    environment:
      CLUSTER_ID: ${KAFKA_INTERNAL_CLUSTER_ID}
      KAFKA_OPTS: "-Djava.security.auth.login.config=/mnt/shared/config/kafka_server_jaas.conf"
    volumes:
      - kafka-internal-1-data:/data/kafka/logs
      - ./internal/kafka1:/mnt/shared/config
    restart: unless-stopped

  # InternalKafka - 节点2
  kafka-internal-2:
    image: apache/kafka:3.7.0
    container_name: kafka-internal-2
    network_mode: kafka-net
    ports:
      - "39092:9092"
    environment:
      CLUSTER_ID: ${KAFKA_INTERNAL_CLUSTER_ID}
      KAFKA_OPTS: "-Djava.security.auth.login.config=/mnt/shared/config/kafka_server_jaas.conf"
    volumes:
      - kafka-internal-2-data:/data/kafka/logs
      - ./internal/kafka2:/mnt/shared/config
    restart: unless-stopped

  # InternalKafka - 节点3
  kafka-internal-3:
    image: apache/kafka:3.7.0
    container_name: kafka-internal-3
    network_mode: kafka-net
    ports:
      - "49092:9092"
    environment:
      CLUSTER_ID: ${KAFKA_INTERNAL_CLUSTER_ID}
      KAFKA_OPTS: "-Djava.security.auth.login.config=/mnt/shared/config/kafka_server_jaas.conf"
    volumes:
      - kafka-internal-3-data:/data/kafka/logs
      - ./internal/kafka3:/mnt/shared/config
    restart: unless-stopped

networks:
  kafka-net:
    driver: bridge

volumes:
  kafka-external-data:
  kafka-internal-1-data:
  kafka-internal-2-data:
  kafka-internal-3-data:
EOF
```

### 7.2 启动集群

```bash
cd /opt/kafka-config

# 生成集群ID
export KAFKA_EXTERNAL_CLUSTER_ID=$(uuidgen)
export KAFKA_INTERNAL_CLUSTER_ID=$(uuidgen)

# 启动所有服务
docker-compose up -d

# 查看状态
docker-compose ps
```

---

## 8. 防火墙配置

```bash
# CentOS/Rocky Linux
sudo firewall-cmd --permanent --add-port=59092/tcp
sudo firewall-cmd --permanent --add-port=29092/tcp
sudo firewall-cmd --permanent --add-port=39092/tcp
sudo firewall-cmd --permanent --add-port=49092/tcp
sudo firewall-cmd --reload

# 或者关闭防火墙（开发环境）
sudo systemctl stop firewalld
sudo systemctl disable firewalld
```

---

## 9. 常见问题排查

### 9.1 端口检查

```bash
# 检查端口是否监听
netstat -tlnp | grep -E '59092|29092|39092|49092'

# 检查容器端口映射
docker port kafka-external
docker port kafka-internal-1
```

### 9.2 查看日志

```bash
# 查看 ExternalKafka 日志
docker logs -f kafka-external

# 查看 InternalKafka 日志
docker logs -f kafka-internal-1
docker logs -f kafka-internal-2
docker logs -f kafka-internal-3
```

### 9.3 常见错误

| 错误 | 原因 | 解决 |
|------|------|------|
| `Controller quorum voters is not valid` | Controller地址配置错误 | 检查 `controller.quorum.voters` 配置 |
| `Replication factor larger than available brokers` | 副本数大于可用节点 | 副本数不能超过节点数 |
| `SASL Authentication failed` | 用户名密码错误 | 检查 JAAS 配置 |
| `Address already in use` | 端口被占用 | 更换端口或关闭占用进程 |

### 9.4 完全清理重试

```bash
# 停止所有容器
docker stop kafka-external kafka-internal-1 kafka-internal-2 kafka-internal-3

# 删除所有容器
docker rm kafka-external kafka-internal-1 kafka-internal-2 kafka-internal-3

# 删除数据卷
docker volume rm kafka-external-data kafka-internal-1-data kafka-internal-2-data kafka-internal-3-data

# 重新创建网络
docker network rm kafka-net
docker network create kafka-net
```

---

## 10. 验证与测试

### 10.1 创建测试 Topic

```bash
docker exec -it kafka-internal-1 bash

cat > /opt/kafka/config/client.properties << 'EOF'
security.protocol=SASL_PLAINTEXT
sasl.mechanism=PLAIN
sasl.jaas.config=org.apache.kafka.common.security.plain.PlainLoginModule required username="admin" password="123456";
EOF

# 创建测试Topic
/opt/kafka/bin/kafka-topics.sh --create \
  --bootstrap-server 192.168.239.164:29092 \
  --replication-factor 3 \
  --partitions 3 \
  --topic test-topic \
  --command-config /opt/kafka/config/client.properties

# 列出所有Topic
/opt/kafka/bin/kafka-topics.sh --bootstrap-server 192.168.239.164:29092 --list --command-config /opt/kafka/config/client.properties
```

### 10.2 生产者测试

```bash
# 发送测试消息
/opt/kafka/bin/kafka-console-producer.sh \
  --bootstrap-server 192.168.239.164:29092 \
  --topic test-topic \
  --producer.config /opt/kafka/config/client.properties
# 输入测试消息后按 Ctrl+C 退出
```

### 10.3 消费者测试

```bash
# 接收测试消息
/opt/kafka/bin/kafka-console-consumer.sh \
  --bootstrap-server 192.168.239.164:29092 \
  --topic test-topic \
  --from-beginning \
  --consumer.config /opt/kafka/config/client.properties
```

---

## 11. 配置 AI-Transform 项目

确保 `dev.config.yaml` 中的 Kafka 配置正确：

```yaml
externalKafka:
  user: admin
  pwd: 123456
  saslMechanism: PLAIN
  maxRetry: 3
  address:
    - 192.168.239.164:59092

kafka:
  user: admin
  pwd: 123456
  saslMechanism: PLAIN
  maxRetry: 3
  address:
    - 192.168.239.164:29092
    - 192.168.239.164:39092
    - 192.168.239.164:49092
```

---

*文档更新时间：2026-03-18*
