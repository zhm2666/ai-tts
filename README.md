# gpt-sovits 
```shell
docker run -d --name gpt-sovits-api-model quay.io/0voice/gpt-sovits-api-model:1.0.0
```
```shell
docker run -d --name gpt-sovits1 --volumes-from gpt-sovits-api-model \
-p 9880:9880 --shm-size="16G" \
-v ai-transform-refer-vol:/app/GPT-SoVITS/runtime/refer \
quay.io/0voice/gpt-sovits-api:1.0.2 python3 api.py -dl zh -d cpu -fp -p 9880
```
# refer api
```shell
docker build -t ai-tranform-refer:0.1.0 -f Dockerfile.referapi .
```
```shell
docker run -d --name tranform-refer -p 8085:8085 \
-v ai-transform-refer-vol:/app/runtime/refer \
-v /home/nick/refer/dev.referapi.config.yaml:/app/config.yaml \
ai-tranform-refer:0.1.0
```

# transform
```shell
docker build -t ai-tranform:0.1.0 -f Dockerfile.transform .
```
```shell
docker volume create ai-transform-vol
```
```shell
docker config create prod-ai-transform-conf ~/dev.config.yaml
```
```shell
docker service create --name ai-transform \
--config src=prod-ai-transform-conf,target=/app/config.yaml \
--replicas 1 \
--mount type=volume,src=ai-transform-vol,dst=/app/runtime \
--with-registry-auth \
ai-tranform:0.1.0
```

# web api
```shell
docker build -t ai-transform-web:0.1.0 -f Dockerfile.web .
```
```shell
docker config create prod-ai-transform-web-conf ~/dev.config.yaml 
```
```shell
docker service create --name ai-transform-web -p 8081:8081 \
--config src=prod-ai-transform-web-conf,target=/app/config.yaml \
--replicas 2  \
--with-registry-auth \
ai-transform-web:0.1.0
```