# Local Cell Watchdog Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 broker 宿主机增加定时自愈 watchdog，持续修复 `local-danted` cell 的 source IPv6 与本地 danted 服务故障。

**Architecture:** 以独立命令形式实现 watchdog，通过 broker admin API 读取和验证 cell，通过宿主机命令修复 `eth0` `/128` 地址和对应的 `danted` systemd unit。broker core 不感知任何宿主机修复细节。

**Tech Stack:** Go 1.24、标准库 `net/http`/`os/exec`/`log/slog`、systemd service/timer

---

### Task 1: 定义纯逻辑边界

**Files:**
- Create: `internal/localcellwatchdog/watchdog.go`
- Test: `internal/localcellwatchdog/watchdog_test.go`

- [ ] **Step 1: 写失败用例**

  覆盖：
  - 只选择 `transport=local-danted && proxy.host=127.0.0.1` 的 active socks5 cell
  - `11080/11082/11083` 到 systemd unit 的映射
  - 缺失 `labels.ipv6`、未知端口时不进入修复

- [ ] **Step 2: 跑测试确认失败**

  Run: `go test ./internal/localcellwatchdog -run Test -count=1`

- [ ] **Step 3: 写最小实现**

  提供：
  - 目标 cell 选择
  - port -> unit
  - 单轮修复决策所需的数据结构

- [ ] **Step 4: 跑测试确认通过**

  Run: `go test ./internal/localcellwatchdog -run Test -count=1`

### Task 2: 实现 watchdog 命令

**Files:**
- Create: `cmd/local-cell-watchdog/main.go`
- Modify: `internal/localcellwatchdog/watchdog.go`
- Test: `internal/localcellwatchdog/watchdog_test.go`

- [ ] **Step 1: 写失败用例**

  覆盖：
  - 初测失败时会尝试补 IPv6、重启服务、复测、清 cooldown
  - 初测成功但有 cooldown 时会清 cooldown
  - 复测失败时不会误清 cooldown

- [ ] **Step 2: 跑测试确认失败**

  Run: `go test ./internal/localcellwatchdog -run TestHeal -count=1`

- [ ] **Step 3: 写最小实现**

  实现：
  - admin API client
  - runner/exec 抽象
  - 单轮检查/修复/复测
  - 结构化日志

- [ ] **Step 4: 跑测试确认通过**

  Run: `go test ./internal/localcellwatchdog -count=1`

### Task 3: 加安装资产

**Files:**
- Create: `ops/systemd/local-cell-watchdog.service`
- Create: `ops/systemd/local-cell-watchdog.timer`
- Create: `scripts/install-local-cell-watchdog.sh`

- [ ] **Step 1: 写最小安装资产**

  资产需要：
  - `/usr/local/bin/local-cell-watchdog` 可执行
  - service 为 oneshot
  - timer 每 2 分钟触发
  - 安装脚本负责 build / copy / daemon-reload / enable --now

- [ ] **Step 2: 做本地静态检查**

  Run: `sed -n '1,220p' ops/systemd/local-cell-watchdog.service ops/systemd/local-cell-watchdog.timer scripts/install-local-cell-watchdog.sh`

### Task 4: 验证与部署

**Files:**
- Modify: `docs/superpowers/specs/2026-04-25-local-cell-watchdog-design.md`

- [ ] **Step 1: 跑完整测试**

  Run: `go test ./internal/localcellwatchdog ./cmd/local-cell-watchdog -count=1`

- [ ] **Step 2: 构建命令**

  Run: `go build ./cmd/local-cell-watchdog`

- [ ] **Step 3: 安装到 `ccc`**

  Run: `bash scripts/install-local-cell-watchdog.sh ccc`

- [ ] **Step 4: 做故障注入验证**

  在 `ccc`：
  - 删除一个 `/128`
  - `systemctl start local-cell-watchdog.service`
  - 验证 cell test 成功、cooldown 清空
