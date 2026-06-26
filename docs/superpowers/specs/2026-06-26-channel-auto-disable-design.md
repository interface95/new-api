# 通道自动禁用：连续失败计数设计

日期：2026-06-26
状态：已确认设计，待写实现计划

## 背景与问题

当前工作区中存在一份**未提交**的「时间窗内累计 N 次失败才禁用」实现：

- `common/constants.go`：`AutomaticDisableFailureThreshold` / `AutomaticDisableFailureWindowSeconds`
- `service/channel_disable_tracker.go`：进程内 `map[string][]time.Time` 滑动时间窗
- `service/channel.go`：`DisableChannel` 接入决策
- 前端 `routing-reliability-section.tsx` 等 + 六语言 i18n

该实现有三个准确性 / 正确性缺陷：

1. **完全不统计成功**。`DisableChannel` 只在出错时被调用（`controller/relay.go:363`、`controller/channel-billing.go:476`），成功请求从不进入计数器。因此它衡量的是「失败的绝对数量」而非「失败率 / 健康度」。一个每分钟上万次成功、偶发一次失败的健康通道，失败会随时间累积并最终被误禁用。
2. **进程内存储，多副本部署下阈值失效**。计数器是 in-memory，HA 多副本时负载均衡把失败分散到各副本，实效阈值变成「阈值 × 副本数」，任何单副本都可能永远到不了阈值，阈值形同虚设。
3. **只按时间间隔索引**。判断只看时间、不看流量，把低频零星失败误判为故障。

## 目标

- 用「**连续失败计数**」取代「时间窗计数」：只要有一次成功就把计数清零，使健康通道几乎不会被误禁。
- 计数存储采用 **Redis 优先、本机内存 fallback**：Redis 可用时集群多副本共享同一计数；Redis 不可用时单实例部署仍能工作。
- 保持向后兼容：阈值默认 1 时，行为与现版本一致（一次 qualifying 失败即禁）。

## 非目标

- 不实现「失败率 + 最小样本」模式（更精确但需统计全部成功量，YAGNI，将来需要再加）。
- 不扩展为完整熔断器的 half-open 主动探活（项目已有 `AutomaticEnableChannelEnabled` 自动恢复，本次不动恢复侧）。
- 不做跨进程的非 Redis fallback。无 Redis 时仅提供**当前进程内**的 best-effort 连续失败计数，多副本部署不保证全局准确。

## 设计概览：连续失败语义

每个 `(channelId, usingKey)` 维护一个**连续失败计数器**：

| 事件 | 动作 |
|---|---|
| qualifying 失败（经 `ShouldDisableChannel` 过滤后的错误） | 计数 +1；若 `计数 >= 阈值` 则禁用通道 |
| 任意一次成功 | 计数清零（删除 Redis key / 内存记录） |
| 手动 / 自动启用通道（`EnableChannel`） | 计数清零（删除 Redis key / 内存记录） |
| 距上次失败超过 TTL | Redis key 自然过期；内存计数在下一次访问时按 TTL 归零（保险，防零星失败跨长时间累加） |

- 阈值 `AutomaticDisableFailureThreshold` 默认 **1**：等价于「一次失败即禁」，与现版本行为一致；管理员调高后才启用「连续 N 次」保护。
- TTL `AutomaticDisableFailureWindowSeconds`（复用现有字段名）默认 **300** 秒：连续失败计数键的过期时间。**每次失败 `INCR` 后都刷新该 TTL**，因此语义是「距**最后一次**失败超过 TTL 无新失败则计数自动归零」（不是从首次失败计时）。

### 存储策略

优先使用 Redis：

- Redis 可用时，失败计数和 reset 都走 Redis，所有应用副本共享同一计数。
- Redis `INCR` 成功后，清掉当前进程内对应 fallback 计数，避免 Redis 恢复后旧内存计数污染后续 fallback。

Redis 不可用时退回本机内存：

- `!common.RedisEnabled`、`common.RDB == nil`，或 Redis 调用失败时，记录 `SysError`，然后使用进程内 map 计数。
- 内存 fallback 使用同一 key 维度和同一 TTL 语义：若距最后一次失败超过 TTL，先归零再计数。
- 成功 / 启用 reset 总是先清本机内存计数，再尽力删除 Redis key。
- 无 Redis 的单实例部署仍能正确工作；多副本部署下每个进程各自计数，只是 best-effort，需在日志和前端文案中明确提示。

### 计数维度（通道级，刻意不含 model）

计数维度为 `(channelId, usingKey)`，**不含 model**。后果：同一 key 对模型 A 成功会清掉它对模型 B 的连续失败计数。这是刻意取舍——自动禁用动作本身是**通道级**的（禁用即停用整个通道/key），通道只要对任一模型还能成功，就说明该 key 仍然活着，不应因某个模型的失败而禁掉整条通道。若将来需要 per-model 路由可靠性，再单独引入 model 维度，本设计不做（YAGNI）。

## 组件与接口

`service/channel_disable_tracker.go` 整体改写。删除当前「时间窗内累计事件列表」的 `map[string][]time.Time` 逻辑，改为「连续失败计数」逻辑：Redis 优先，内存 fallback。

```go
// 记录一次失败，返回累计连续失败数和实际使用的存储后端（redis / memory）。
// Redis 未启用或调用出错时自动使用内存 fallback。
func incrChannelFailure(key string, ttl time.Duration) (count int, backend string)

// 成功 / 启用时清零，异步执行，忽略 Redis 错误。
// 始终清本机内存 fallback；Redis 可用时再尽力删除 Redis key。
func resetChannelFailure(key string)
```

- `incrChannelFailure` 调用新增的 Redis 原子封装（见下节）。
- `incrChannelFailure` 在 Redis 不可用或出错时调用内存 fallback；调用方无需再处理 Redis 错误分支。
- `resetChannelFailure` 总是删除本机内存 fallback 计数；有 Redis 时再走 `common.RedisDel`，错误只记日志不向上抛。
- key 维度：`autoDisableFailureKey(channelId, usingKey)` 改造为加固定前缀 `auto_disable_failure:`，且 **usingKey 绝不放原文**——`usingKey` 是上游真实密钥（`ContextKeyChannelKey`，见 `middleware/distributor.go:480`），若放进 Redis key 名会随 debug log / `MONITOR` / dump / 备份泄露。改用 `sha256(usingKey)` 的十六进制前 16 字符。最终形如 `auto_disable_failure:channel:<id>`（单 key 通道，usingKey 为空时）或 `auto_disable_failure:channel:<id>:key:<sha256(usingKey)[:16]>`。
- 纯判定逻辑抽成可测函数：`func shouldDisableByFailureCount(count, threshold int) bool`。

## Redis 原子计数封装

现有 `common.RedisIncr` 不可用：它不返回新值，且 key 不存在时直接 `return nil` 不创建。`common/redis.go` 新增基于 Lua 的原子封装（项目当前使用 `github.com/go-redis/redis/v8`，`redis.NewScript` 可用）：

```go
// INCR；每次都刷新 TTL（滑动过期，从最后一次失败计时）；返回自增后的值。
var failureIncrScript = redis.NewScript(`
  local n = redis.call('INCR', KEYS[1])
  redis.call('EXPIRE', KEYS[1], ARGV[1])
  return n
`)

func RedisIncrWithTTL(key string, ttlSeconds int) (int64, error)
```

一次往返、原子，集群共享。**每次 INCR 都 EXPIRE**，保证「距最后一次失败超过 TTL 才归零」（P1 修正：旧设计仅首次 EXPIRE，会变成从首次失败计时）。仅当 `RedisEnabled && RDB != nil` 时可用，否则由 tracker 自动进入内存 fallback。

## 接入点（6 处）

1. **失败** — `service/channel.go: DisableChannel`
   现有 `recordChannelAutoDisableFailure` 改为调 `incrChannelFailure`。
   - Redis 可用 → Redis 计数；Redis 不可用 / 出错 → 本机内存 fallback 计数。
   - `计数 < 阈值` → 记 `SysLog`（命中规则但暂不禁用）并返回。
   - `计数 >= 阈值` → 维持现有 `model.UpdateChannelStatus(... ChannelStatusAutoDisabled ...)` 与通知逻辑；reason 追加 `"N/threshold consecutive failures"`。
   - 日志包含本次使用的后端（`redis` / `memory`），方便排查 Redis 是否失效。
   - 该函数已由 `controller/relay.go:363` 经 `gopool.Go` 异步调用，无需额外并发处理。

2. **确定性直接禁用** — `service/channel.go`
   新增私有 `disableChannelNow(channelError, reason)` 承载现有 `model.UpdateChannelStatus(... ChannelStatusAutoDisabled ...)` 与通知逻辑；`DisableChannel` 在连续失败阈值命中后调用它。
   - `controller/channel-billing.go:476` 的 `"余额不足"` 属于确定性状态，不参与连续失败阈值，改为调用 `service.DisableChannelImmediately(...)`（或等价公开函数，内部走 `disableChannelNow`）。
   - 这样管理员把阈值调高后，临时 upstream 错误会被连续失败保护；余额不足仍会立即停用，避免继续路由到已耗尽余额的通道。

3. **成功** — `controller/relay.go:226` 成功分支（`newAPIError == nil`）
   新增 `service.RecordChannelSuccess(channel.Id, usingKey)` → `resetChannelFailure`。
   - `usingKey` 取 `common.GetContextKeyString(c, constant.ContextKeyChannelKey)`，与错误路径口径一致。
   - 用 `gopool.Go` 异步执行，不拖慢成功热路径。

4. **启用** — `service/channel.go: EnableChannel`
   现有 `channelAutoDisableFailures.clear(...)` 改为 `resetChannelFailure(autoDisableFailureKey(channelId, usingKey))`。

5. **定时通道测试** — `controller/channel-test.go:~930`（P1 修正）
   定时测试失败会走 `DisableChannel`（即 `incr`），但测试**成功**当前只在 `!isChannelEnabled` 时才 `EnableChannel`；通道仍 enabled 时测试成功**不会**清零，导致「测试失败 → 测试成功 → 测试失败」被误当连续失败。
   修正：在响应时间阈值检查之后，若测试结果是真成功（`newAPIError == nil && !shouldBanChannel`），**无论通道是否已 enabled** 都调 `service.RecordChannelSuccess(channel.Id, usingKey)` 清零。`usingKey` 取 `common.GetContextKeyString(result.context, constant.ContextKeyChannelKey)`。
   - 注意：`!service.ShouldDisableChannel(result.newAPIError)` 不能当成功条件，因为非禁用类错误也会返回 false，不应清零连续失败计数。

6. **Midjourney 无实例账号** — `relay/mjproxy_handler.go:~589`
   当前 `midjResponse.Code == 3` 且自动禁用开启时直接 `model.UpdateChannelStatus(..., 2, ...)`，会绕过连续失败阈值，且 `2` 是手动禁用状态。
   修正：改为走 `service.DisableChannel(...)`，让该类自动禁用同样受连续失败阈值控制，并统一写入 `ChannelStatusAutoDisabled` 与通知逻辑。

## 降级策略：Redis 优先 + 内存 fallback

连续失败计数优先使用 Redis；Redis 不可用时自动退回本机内存计数。禁用通道是破坏性操作，本设计选择「尽量保护单机可用，同时把多副本准确性风险显式暴露」：

- **Redis 未启用**（`!common.RedisEnabled`）、`common.RDB == nil` 或 Redis 调用失败：记 `SysError`，然后使用本机内存 fallback 继续计数，不跳过自动禁用逻辑。
- **成功 / 启用清零** 总是清本机内存 fallback；Redis 可用时再尽力 `DEL` Redis key，Redis 删除失败只记日志，不影响请求。
- **显式提示（P2 修正）**：
  - 启动时在 `common.InitRedisClient()` 之后检查：若 `AutomaticDisableChannelEnabled == true` 但 `!RedisEnabled`，记一条醒目的 `SysError`：自动禁用正在使用单实例内存 fallback，多副本部署不保证全局准确。
  - `model.UpdateOption("AutomaticDisableChannelEnabled", "true")` 热更新开启该功能时也做同样检查，避免运行中打开开关却没有任何提示。
  - 前端 `routing-reliability-section.tsx` 的自动禁用区块加一句说明文案（i18n）：「Redis 可用时连续失败计数在多副本间共享；未配置 Redis 时退回单实例内存计数，多副本不保证准确」。
- 已知后果：**无 Redis 的多副本部署下，阈值按进程分别计算**。这比完全不生效更适合单机和小站点，但不提供全局准确性。

## 配置项与向后兼容

- 后端：
  - `AutomaticDisableFailureThreshold` 默认 1，保留语义（连续失败阈值）。
  - `AutomaticDisableFailureWindowSeconds` 保留字段名，语义改为「连续失败计数 TTL（秒）」，默认 300。`model/option.go` 的解析与下限校验（`< 1` 归 1）保留。
- 前端 `web/default/src/features/system-settings/models/routing-reliability-section.tsx`：
  - 第二个数字输入框的 `FormLabel` / `FormDescription` 由「窗口秒数」改为「连续失败计数过期时间（TTL，秒）」相应文案。
  - zod 校验范围不变（threshold 1–100，TTL 1–86400）。
  - `types.ts` / `index.tsx` / `section-registry.tsx` / `model-mutate-drawer.tsx` 字段不变。
- i18n：`web/default/src/i18n/locales/{en,zh,fr,ru,ja,vi}.json` 同步改名后的文案 key/值。

## 错误处理

- Redis 计数错误不向上抛、不影响请求主流程；tracker 记录 `SysError` 后使用内存 fallback。
- `resetChannelFailure` 异步、忽略 Redis 错误；内存 fallback 必须同步清除（清零失败最多导致一次多余的失败累加，由 TTL 兜底）。

## 测试策略

- 后端：
  - `shouldDisableByFailureCount(count, threshold)` 确定性表测试：阈值边界（count == threshold-1 / == threshold / > threshold）、threshold 下限。
  - `autoDisableFailureKey` 测试：验证单 key / 多 key 维度，且**断言原始 usingKey 不出现在结果中**（usingKey 经 sha256，回归保护 P1-c）。
  - **Redis 行为测试（P2 修正，引入 `github.com/alicebob/miniredis/v2`）**——这是本次最易错的部分，必须覆盖：
    - **TTL 刷新**：连续两次 `RedisIncrWithTTL`，断言第二次后 key 的 TTL 被刷新到接近满值（验证「从最后一次失败计时」，回归保护 P1-a）。
    - **内存 fallback**：模拟 Redis 不可用 / 未启用时 `incrChannelFailure` 走 memory backend，达到阈值后仍会禁用。
    - **Redis 错误 fallback**：模拟 Redis 调用失败时记录错误并走 memory backend。
    - **reset cleanup**：模拟 `RedisEnabled=false` / `RDB=nil` 时调用 `RecordChannelSuccess` / `resetChannelFailure` 不 panic，且本机内存计数被清除。
    - **成功 reset**：incr 到阈值前一步 → reset → 再 incr，断言计数从 1 重新开始。
    - **直接禁用旁路**：余额不足调用 `DisableChannelImmediately` 后仍立即写入自动禁用，不受连续失败阈值影响。
    - **定时测试成功条件**：只有 `newAPIError == nil && !shouldBanChannel` 才 reset；非禁用类错误不 reset。
  - `channel_disable_tracker_test.go` 改写：删除针对内存时间窗的旧用例。
  - 使用 `testify/require` 做 setup/fatal、`testify/assert` 做值校验。
- 前端：
  - 更新 `routing-reliability-section.test.ts`：覆盖改名后字段的默认值（threshold=1、TTL=300）与校验边界。

## 受影响文件清单

改写 / 修改：
- `common/redis.go`（新增 `RedisIncrWithTTL` + 每次刷新 TTL 的 Lua 脚本）
- `service/channel_disable_tracker.go`（整体改写为 Redis 优先 + 内存 fallback；key 含 sha256(usingKey)）
- `service/channel.go`（`DisableChannel` / `DisableChannelImmediately` / `EnableChannel` / `RecordChannelSuccess` 接入）
- `controller/relay.go`（成功分支新增 `RecordChannelSuccess`）
- `controller/channel-test.go`（定时测试成功路径新增 reset，P1-b）
- `controller/channel-billing.go`（余额不足改走直接禁用旁路，不受连续失败阈值影响）
- `relay/mjproxy_handler.go`（Midjourney 无实例账号改走 `service.DisableChannel`，不再直接写状态 2）
- `common/constants.go`（TTL 语义注释，字段不变）
- `main.go`（`InitRedisClient()` 后做无 Redis fallback 启动提示）
- `model/option.go`（TTL 语义注释；热更新开启 `AutomaticDisableChannelEnabled` 时做无 Redis fallback 提示）
- `web/default/src/features/system-settings/models/routing-reliability-section.tsx`（改名文案 + fallback 提示）
- `web/default/src/i18n/locales/{en,zh,fr,ru,ja,vi}.json`（改名文案 + fallback 提示）
- `go.mod` / `go.sum`（新增 `github.com/alicebob/miniredis/v2` 测试依赖，P2-d）
- `service/channel_disable_tracker_test.go`、`common/redis_test.go`、`routing-reliability-section.test.ts`（测试改写 / 新增）

不变（沿用现有未提交改动）：
- `types.ts` / `index.tsx` / `section-registry.tsx` / `model-mutate-drawer.tsx` 的字段声明
- `model/option_test.go`（如覆盖 option 解析，按需微调）

## 风险与权衡

- **成功 reset 与失败 incr 的并发竞态 —— 接受近似计数（P2-b）**：成功分支异步 `DEL` 与失败分支异步 `INCR` 跨并发请求无全局顺序。极端时序下，较早请求的成功 `DEL` 可能晚于较晚请求的失败 `INCR` 执行，擦掉一次新的失败计数，使实际触发禁用比阈值略晚。**本设计有意接受此近似**：自动禁用是容错保护场景，偶尔多放行一两次失败不影响正确性，TTL 也会兜底。不引入带逻辑时钟/序号的复杂 Lua 脚本（YAGNI）。
- **内存 fallback 在多副本下不全局准确**：无 Redis 时每个进程独立计数，阈值可能被副本数稀释。已通过启动日志和前端文案显式提示。
- **Redis 故障期间可能切换计数后端**：Redis 失败时内存 fallback 从本进程本地状态开始计数；Redis 恢复后重新使用 Redis 计数。成功 reset 会同时清本机内存和 Redis，降低状态残留风险。
- **每次成功一次 `DEL`**：高 QPS 下是额外 Redis 调用，但异步执行且为 O(1)；若将来成为热点，可加「本地短期标记，仅在近期见过失败时才 DEL」优化，本次不做（YAGNI）。
- **默认阈值 1** 意味着新「连续失败」保护默认不启用，需管理员主动调高——可接受。
