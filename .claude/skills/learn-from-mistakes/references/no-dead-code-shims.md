---
match: // deprecated|// TODO.*remove|_old_|_legacy|_v[0-9]
action: inject
match_on: new
---

# No dead code or compatibility shims

## Symptom

代码库膨胀：`_old_` 后缀的函数、`// deprecated` 注释的方法、`_v2` 命名的替代实现。后续修改时不确定该用哪个版本。

## Root Cause

个人项目沿用了库/框架的兼容性策略。broker 没有外部消费者，不需要任何 deprecation 过渡期。

## Correct Approach

1. **直接删除旧代码**，Git 记住一切，需要时 `git log -p` 找回
2. 不要写 `// deprecated`、`// TODO: remove in v2`、`_old_`、`_legacy`
3. 重构时检查 ripple effects：
   - 删除函数 → 检查所有 caller
   - 修改签名 → 更新所有调用点
   - 删除类型 → 删除相关的 helper、test、mock
4. 不要给未使用的变量加 `_` 前缀来消除编译错误 — 直接删除
5. 不要 re-export 已删除的类型来保持兼容 — 更新所有 import
