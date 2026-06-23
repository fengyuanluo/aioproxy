# 发布

GitHub Actions 策略：

- `push` 到 `main`：运行测试，构建 Linux/macOS/Windows 的 amd64/arm64 包；通过后自动更新 `continuous` prerelease，上传打包产物和 checksums。
- `pull_request`：运行测试和构建验证，不发布。
- `v*` tag：创建或更新对应版本的 GitHub Release，上传打包产物和 checksums，并作为稳定版本。

目标矩阵：

- `linux/amd64`
- `linux/arm64`
- `darwin/amd64`
- `darwin/arm64`
- `windows/amd64`
- `windows/arm64`

`continuous` release 是 main 分支最新构建，tag release 才是稳定正式分发渠道。当前 workflow 不再依赖 Actions artifacts 作为发布中转，避免 artifact 存储配额阻塞发版。
