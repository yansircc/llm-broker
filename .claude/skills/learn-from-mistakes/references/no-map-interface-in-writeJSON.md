---
match: map\[string\]interface[{][}]
action: inject
match_on: new
---

# No ad-hoc map responses

## Symptom

前端收到 `sessions: null` 或字段名不一致，导致 UI crash。API 响应结构无法静态检查，前端只能靠猜。

## Root Cause

用 `map[string]interface{}` 拼装 JSON 响应：
- 字段名靠字符串拼写，typo 无法编译期发现
- nil slice 在 map 中序列化为 `null`（见 `nil-slice-becomes-null`）
- 无法 grep 出某个 API 返回了哪些字段
- 前端和后端之间没有契约

## Correct Approach

1. **定义 typed DTO struct**（在 handler 文件或 `domain/` 包中），用 `json:"field_name"` tag 明确字段名
2. 所有 `writeJSON` / `json.Marshal` 调用只接受 typed struct
3. AST lint (`lint_test.go`) 已禁止 `map[string]interface{}` 出现在 `writeJSON` 参数中
4. slice 字段用 `make([]T, 0)` 初始化，避免 null（见 `nil-slice-becomes-null`）
