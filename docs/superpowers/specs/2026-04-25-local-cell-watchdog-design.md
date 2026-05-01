# Local Cell Watchdog Design

**日期：** 2026-04-25

## 目标

为 `labels.transport=local-danted` 且 `proxy.host=127.0.0.1` 的 broker egress cell 增加宿主机侧 watchdog，持续保证：

- cell 的真实连通性不丢
- 宿主机 `/128` source IPv6 不因网络重配而失效
- 因链路故障产生的 cell cooldown 在恢复后自动收口

## 稳定轴 / 变化轴 / 不变量

- 稳定轴：cell 可用性的真相是 `POST /admin/egress/cells/{id}/test`
- 变化轴：`eth0` 上的 `/128` 地址、本地 `danted-*` 进程、cell cooldown
- 不变量：`active` 的 `local-danted` cell 必须可通过 admin test 成功拨通；恢复后不应继续保留过期 cooldown

## 边界

这不是 broker core 逻辑。

- watchdog 运行在 broker 宿主机
- 通过 broker admin API 读取/验证/清理 cell 状态
- 通过宿主机命令修复网络与 `danted` 服务
- 不进入 `pool` / `relay` / `driver` 增加 provider 特例
- 不理解账号、模型、provider 协议

## 输入真相

- cell 列表：`GET /admin/egress/cells`
- 可用性探针：`POST /admin/egress/cells/{id}/test`
- cooldown 收口：`POST /admin/egress/cells/{id}/clear-cooldown`
- source IPv6：cell `labels.ipv6`
- 本地监听端口：cell `proxy.port`

## 选择规则

仅处理满足下列条件的 cell：

- `status == active`
- `proxy.type == socks5`
- `proxy.host == 127.0.0.1`
- `labels.transport == local-danted`
- `labels.ipv6` 非空
- `proxy.port > 0`

## 自愈流程

每轮 watchdog 对目标 cell 逐个执行：

1. 先做 cell test
2. 若成功：
   - 若 cell 有 cooldown，则调用 clear-cooldown
   - 记录恢复日志
3. 若失败：
   - 检查 `eth0` 是否已挂载 `labels.ipv6/128`
   - 若缺失则补回 `/128`
   - 按约定映射出本地 `danted-*` unit 并重启
   - 再次执行 cell test
4. 复测成功：
   - 清理 cooldown
   - 记录自愈成功日志
5. 复测失败：
   - 保留失败状态
   - 记录结构化错误日志

## unit 映射

本次只覆盖当前已知本地 unit：

- `11080 -> danted-linda.service`
- `11082 -> danted-cell-uk-linode-02-local.service`
- `11083 -> danted-cell-uk-linode-03-local.service`

后续若新增 local cell，应把“port -> unit”映射扩展到同一位置，不分散。

## 运行形态

- 仓库内实现一个独立命令：`cmd/local-cell-watchdog`
- 宿主机安装为 `/usr/local/bin/local-cell-watchdog`
- 用 `systemd` 提供：
  - `local-cell-watchdog.service`
  - `local-cell-watchdog.timer`
- 周期：每 2 分钟
- 允许手动立即触发：`systemctl start local-cell-watchdog.service`

## 可观测性

日志要求：

- 结构化 key-value
- 至少包含：`cell_id`、`cell_name`、`ipv6`、`proxy_port`、`service_name`、`stage`、`result`
- 自愈前后要有因果链：初测失败 -> 修复动作 -> 复测结果 -> cooldown 清理结果

## 测试

### 自动化

- 纯逻辑测试：
  - cell 筛选
  - port -> unit 映射
  - 成功/失败/复测成功的决策路径

### 集成验证

在 `ccc` 上做人为故障注入：

- 手工删掉 `eth0` 上某个 `/128`
- 运行 watchdog
- 验证地址补回、对应 `danted` 重启、cell test 变为成功、cooldown 被清理

## 非目标

- 不做 provider 级健康判定
- 不修改 broker 调度逻辑
- 不把宿主机网络修复逻辑塞进 broker core
- 不引入无限重试或守护常驻进程
