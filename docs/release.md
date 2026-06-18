# 发布

GitHub Actions 策略：

- `push` 到 `main`：运行测试，构建 Linux/macOS/Windows 的 amd64/arm64 包，上传 CI artifacts。
- `pull_request`：运行测试和构建验证，不发布。
- `v*` tag：创建 GitHub Release，上传打包产物和 checksums。

目标矩阵：

- `linux/amd64`
- `linux/arm64`
- `darwin/amd64`
- `darwin/arm64`
- `windows/amd64`
- `windows/arm64`

CI artifacts 不是正式版本；tag release 才是正式分发渠道。
