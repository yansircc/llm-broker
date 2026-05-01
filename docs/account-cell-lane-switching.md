# 账号 Surface 切换说明

这件事切的不是账号字段，而是账号绑定的 egress cell 上的 `labels.lane`。

## 核心事实

- stable axis: surface 调度逻辑在 `cell.labels.lane`
- change axis: `lane` 取值从 `<unset>` / `compat` / `all` 之间切换
- invariant: 必须通过当前 active slot 的 admin API 更新，不能直接改 SQLite

当前逻辑是：

- `lane=<unset>`: 默认 `native-only`
- `lane=compat`: `compat-only`
- `lane=all`: 同时接受 `native` 和 `compat`

不要把这件事理解成“给账号改一个 compat 开关”。账号自己没有这个字段。

## 风险边界

- 这是 cell 级切换，不是账号级切换。
- 如果一个 cell 绑了多个账号，改 `lane` 会一起影响该 cell 上的所有账号。
- 只有在目标账号绑定的是独占 cell 时，才适合把它当成“给某个账号切换 surface”。
- 不要直接改数据库。直接改 SQLite 不会刷新活跃进程内存态，蓝绿下还容易改到不生效实例。
- 蓝绿部署下只改当前 active slot。inactive slot 不对外提供服务，改它没有立即效果。

## 推荐做法

用仓库脚本：

```bash
scripts/set-account-cell-lane.sh --remote ccc --account kun --lane unset
scripts/set-account-cell-lane.sh --remote ccc --account kun --lane compat
scripts/set-account-cell-lane.sh --remote ccc --account kun --lane all
```

参数含义：

- `--remote`: SSH 别名或主机，例如 `ccc`
- `--account`: 账号 `email` 或账号 `id`
- `--lane unset`: 删除 `lane`，回到默认 `native-only`
- `--lane compat`: 切成 `compat-only`
- `--lane all`: 双开 `native` + `compat`

脚本行为：

- 自动读取远端 `/var/lib/llm-broker/bluegreen/active-slot`
- 自动命中 active slot 对应端口
- 通过 admin API 读取账号和 cell 当前状态
- 默认拒绝修改共享 cell
- 只改 `labels.lane`，保留该 cell 其余 `name/status/proxy/labels`
- 写入后回读账号可用面，输出 `available_native` / `available_compat`

如果明确知道目标 cell 是共享的，并且你就是要一起改，可以显式加：

```bash
scripts/set-account-cell-lane.sh --remote ccc --account kun --lane compat --allow-shared-cell
```

## 为什么 `native` 推荐用 `unset`

代码里 `native` 和 `<unset>` 对 native surface 都是可用的，但 `<unset>` 状态更干净，符合项目一贯偏好：

- 少一个多余状态
- 少一层人为约定
- 更接近“默认 native”的真实语义

所以脚本虽然接受 `--lane native`，实际会把它归一化成删除 `lane`。

## 例子

把 `kun` 从 `compat-only` 切回 `native-only`：

```bash
scripts/set-account-cell-lane.sh --remote ccc --account kun --lane unset
```

把 `kun` 恢复成 `compat-only`：

```bash
scripts/set-account-cell-lane.sh --remote ccc --account kun --lane compat
```

把 `kun` 开成双面：

```bash
scripts/set-account-cell-lane.sh --remote ccc --account kun --lane all
```

## 验证

脚本执行后会直接输出：

- 当前 active slot 和命中的端口
- 目标账号 / cell
- `lane` 的前后变化
- 写入后的 `available_native`
- 写入后的 `available_compat`

如果你要手动复核，可以再跑一次 dry-run：

```bash
scripts/set-account-cell-lane.sh --remote ccc --account kun --lane compat --dry-run
```

`dry-run` 只做读取和解析，不会实际写入。
