# 持久化与快照

AIOPROXY 持久化：

- Candidate Pool
- source/import snapshots

不持久化：

- session bindings

状态文件带版本号。v1 内升级尽量兼容读取旧状态；无法安全读取时会备份旧文件、重建空状态、继续启动并进入 degraded，等待后续刷新补池。

刷新是 additive：新验活通过的候选合并进当前池，不覆盖整个池。snapshot 按来源保留最近 `storage.snapshot_retention` 份。
