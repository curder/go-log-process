# Go 日志处理

## influxDB

### Docker 安装并启动
```
docker run --name influxdb -p 9999:9999 quay.io/influxdb/influxdb:2.0.0-beta
```
默认登录本地的9999端口：`http://127.0.0.1:9999`，初始化一个用户名和密码，组织名、授权Token以及Bucket