# Go 日志处理

## influxDB

### Docker 安装并启动
```
docker run --name influxdb -p 9999:9999 quay.io/influxdb/influxdb:2.0.0-beta
```
默认登录本地的9999端口：`http://127.0.0.1:9999`，初始化一个用户名和密码，组织名、授权Token以及Bucket

### grafana 的安装并启动

```
docker run  -d --name grafana -p 3000:3000 -v ${PWD}/data:/var/lib/grafana -e "GF_SECURITY_ADMIN_PASSWORD=aaaaaa" grafana/grafana
```
> 设置了默认的密码是 `aaaaaa`，默认的用户名是 `admin`
