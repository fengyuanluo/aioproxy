# 部署

v1 发布多平台二进制，不发布 Docker 镜像，不提供 installer。

## 本地运行

```bash
cp examples/config.yaml config.yaml
./aioproxy check -c config.yaml
./aioproxy serve -c config.yaml
```

## systemd

示例文件：`deploy/systemd/aioproxy.service`。

默认数据与日志路径是相对路径，systemd 必须设置明确 `WorkingDirectory`，或在 YAML 中改成绝对路径。

```bash
sudo install -d /opt/aioproxy
sudo cp aioproxy config.yaml /opt/aioproxy/
sudo cp deploy/systemd/aioproxy.service /etc/systemd/system/aioproxy.service
sudo systemctl daemon-reload
sudo systemctl enable --now aioproxy
```
