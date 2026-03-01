---
match: interface.*[{]|type.*=.*[{]
action: inject
match_on: new
paths: web/src/
---

# Frontend interfaces must match backend JSON tags exactly

## Symptom

页面白屏或 `Cannot read properties of undefined (reading 'replace')`：前端 interface 用 PascalCase（`r.Model`），API 返回 snake_case（`r.model`），JavaScript 属性访问区分大小写，所有字段都是 `undefined`。

## Root Cause

来自 commit `18ba95d` 和后续修复的反复教训。Go struct 的 `json:"snake_case"` tag 决定了 API 的字段名。前端 TypeScript interface 必须精确匹配这些 JSON 字段名，否则静默返回 `undefined`——不会有编译错误或运行时警告。

已发生三次：
1. accounts detail page — `errorMessage` → `error_message`（commit 18ba95d）
2. accounts detail page — `sessions: null` crash（commit 875648d）
3. users detail page — `RecentRequest` 全部 PascalCase → crash（当前修复）

## Correct Approach

1. **前端 interface 字段名必须与 Go struct 的 `json:"..."` tag 完全一致**
2. 修改后端 `json` tag 或 DTO struct 时，**同一 commit** 更新所有消费该 API 的前端文件
3. `responses.go` 中的 typed DTO 是 single source of truth
4. 用 `grep -r` 搜索旧字段名确认无遗漏：`grep -r "PascalField" web/src/`
