# 渠道成功率状态条 — 设计文档

- 日期: 2026-07-02
- 状态: 已修订待实现
- 作者: AI 辅助 (Claude) + 项目维护者确认

## 1. 背景与目标

渠道管理页 (`渠道`) 的每个渠道卡片目前只有主动测试得到的 `响应 (response_time)` / `上次测试 (test_time)` 两个**单点**数据,无法反映渠道在**真实流量**下的健康度。

近期上线的「模型性能状态条」(`ModelPerfBadge`,14 格分段条 + 成功率百分比,数据来自 `pkg/perf_metrics`) 提供了成熟的可视化,但它按 `(model, group)` 聚合,**没有渠道维度**。

**目标**: 给每个渠道卡片加一条 14 格分段状态条 + 成功率百分比,复用模型状态条的视觉与染色,展示该渠道真实流量的成功率健康度。

## 2. 关键决策(已与维护者逐条确认)

| 维度 | 决策 | 理由 |
|---|---|---|
| 统计/展示粒度 | **渠道级一条** | 对应卡片布局,最直接 |
| 失败口径 | **每次调用独立统计**(重试循环内每次尝试都记) | 渠道A挂了被重试救回也要记 A 一次失败,真实反映渠道本身健康度 |
| 错误范围 | **只算渠道侧错误** | 用户侧 4xx(参数非法/余额不足)不该拉低渠道成功率 |
| 存储粒度 | **只按 `(channel_id, bucket_ts)`** | YAGNI;展示就是渠道级一条,不做模型下钻 |
| 展示内容 | **只成功率条 + %** | 卡片紧凑;延迟已有现成「响应」字段 |
| 配置 | **独立 `channel_metrics_setting`**(独立 Enabled,默认对齐 perf_metrics) | 渠道统计写入量可能大,可单独关 |
| 出数接口 | **内联进 `GET /api/channel`** | 前端零额外请求 |
| 覆盖范围 | **普通 relay + task relay** | 二者都走渠道选择/重试/自动禁用路径,都属于渠道真实流量 |

## 3. 为什么不复用现有机制

- **不复用 `perf_metrics` 的同一条记录**: 模型成功率是「最终结果」口径(失败只在整个重试循环全部失败后记一次,归给最后一个渠道,见 `controller/relay.go:248`);渠道成功率要「每次尝试」口径。两者记录时机不同,塞进同一条会互相污染。因此需要独立收集层,但可**照搬** `perf_metrics` 的架构。
- **不从 `logs` 表聚合**: `logs` 的失败 = `Type=LogTypeError`,是最终失败口径,重试救回的尝试不落 Error 日志,无法满足「每次尝试」;且难以区分渠道侧 vs 用户侧错误。

## 4. 架构

分层与现有代码一致: Router → Controller → Service/pkg → Model。新增一个独立收集层,尽量复用 `perf_metrics` 的成熟机制,**不重构** `perf_metrics`(它刚上线且稳定)。

### 4.1 数据模型 — `model/channel_metric.go`

```go
type ChannelMetric struct {
    Id             int   `json:"id" gorm:"primaryKey"`
    ChannelId      int   `json:"channel_id" gorm:"uniqueIndex:idx_chan_metric_channel_bucket,priority:1;index:idx_chan_metric_channel_bucket_lookup,priority:1"`
    BucketTs       int64 `json:"bucket_ts" gorm:"uniqueIndex:idx_chan_metric_channel_bucket,priority:2;index:idx_chan_metric_channel_bucket_lookup,priority:2;index:idx_chan_metric_bucket_ts"`
    RequestCount   int64 `json:"-" gorm:"default:0"`
    SuccessCount   int64 `json:"-" gorm:"default:0"`
    TotalLatencyMs int64 `json:"-" gorm:"default:0"`
}
```

- 唯一键 `(channel_id, bucket_ts)`;成功率 = `SuccessCount / RequestCount`。
- 提供 `UpsertChannelMetric`(原子增量,`ON CONFLICT` 走唯一键)、`GetChannelMetricsBuckets(channelIds, startTs, endTs)`、`DeleteChannelMetricsBefore(ts)`。
- 需要独立 `bucket_ts` 前导索引 `idx_chan_metric_bucket_ts`,保证 `DeleteChannelMetricsBefore(ts)` 这类清理查询不会依赖 `(channel_id, bucket_ts)` 复合索引的非前导列。
- 跨三库: 纯 GORM,不使用 `SERIAL`/`AUTO_INCREMENT`,无保留字列;`ON CONFLICT` 走 GORM `clause.OnConflict`,与 `model/perf_metric.go` 的 `UpsertPerfMetric` 同一写法。

### 4.2 收集层 — `pkg/channel_metrics`(平行 `perf_metrics`)

照搬已验证的管线,只保留渠道需要的三个计数:

- 内存 `sync.Map` 原子热桶: `atomicBucket{ requestCount, successCount, totalLatencyMs }`(`perf_metrics` 那份的子集)。
- Redis 镜像为可选增强: key `chanperf:{channelId}:{bucketTs}`,hash 字段 `req/ok/lat`。首版如果没有实现 Redis 读/flush 去重闭环,就不写 Redis 旁路,避免产生无人消费的镜像数据。
- `flushLoop`: 按配置 flush interval 把已完成热桶 drain 进 `channel_metrics` 表 upsert;保留期外清理。
- 查询合并 DB 桶 + 本进程内存热桶;如果实现 Redis active bucket 合并,必须保持不重复计入本进程热桶。
- 对外 API:
  - `RecordChannelSample(channelId int, success bool, latencyMs int64)` — 记录点调用。
  - `QueryChannelSummary(channelIds []int, hours int) map[int]ChannelSummary` — 合并 DB 桶 + 内存热桶,产出每渠道 `{ SuccessRate, RecentSuccessRates []float64(最近 14 桶), LatestBucketTs }`。`AvgLatencyMs` 首版不返回,避免引入失败样本延迟分母口径争议。
- 成功率 / `recentSuccessRates` 等**纯计算函数**从 `perf_metrics` 复用(提取为可共享的纯函数,不改其行为)。

### 4.3 记录点 — `controller/relay.go` 重试循环**内**

这是与模型口径的本质区别: 记录发生在**每次对某渠道的尝试**处,而非循环外。

#### 4.3.1 普通 relay

- **成功分支** (`newAPIError == nil`,现有 `RecordChannelSuccess` 旁):
  `gopool.Go(func(){ channelmetrics.RecordChannelSample(channel.Id, true, attemptLatencyMs) })`
  `attemptLatencyMs` 按**本次尝试**计时(本次尝试开始到成功),不是整请求。
- **失败分支** (`processChannelError` 旁): 若 `isChannelSideError(newAPIError)` 为真 →
  `gopool.Go(func(){ channelmetrics.RecordChannelSample(channel.Id, false, 0) })`;
  纯用户侧错误 → 既不记成也不记败(分母也不含)。
- 循环外原有模型口径的 `RecordRelaySample(relayInfo, false, 0)`(`controller/relay.go:248`) **原样保留不动**。
- 全部异步 `gopool.Go`,不阻塞 relay 主链路。

#### 4.3.2 Task relay

`RelayTaskSubmit` 路径同样走渠道选择/重试/`RecordChannelSuccess`/`processChannelError`,不能漏统计:

- **成功分支** (`taskErr == nil`,现有 `RecordChannelSuccess` 旁): 记录该 `channel.Id` 一次成功,`attemptLatencyMs` 从本次 `relay.RelayTaskSubmit` 调用前后计算。
- **失败分支** (`!taskErr.LocalError` 且进入 `processChannelError` 的同一处): 将 task 错误转换为 `types.NewAPIError` 后走渠道侧错误判定;渠道侧错误记一次失败,本地读 body/路由/参数类错误不计入。
- `getChannel` 失败没有具体渠道 id,不记录渠道指标。
- `shouldRetryTaskRelay` 仍只负责是否继续重试;指标 helper 不读取 `retryTimes` / `specific_channel_id`,避免把“是否还有重试机会”误当成“错误归因”。

### 4.4 渠道侧错误判定 — `isChannelSideError(err *types.NewAPIError) bool`

复用 `shouldRetry` 已用的错误本质判定,剥离掉 `retryTimes` / `specific_channel_id` 等非错误因素:

- `types.IsChannelError(err)` 为真 → 渠道侧(明确的渠道错误标记)。
- `types.IsSkipRetryError(err)` 为真 → 非渠道侧(跳过)。
- `operation_setting.IsAlwaysSkipRetryCode(err.GetErrorCode())` → 非渠道侧。
- 否则按 `StatusCode`: 2xx → 否;越界码(<100 或 >599)→ 是;其余 `operation_setting.ShouldRetryByStatusCode(code)`。

抽成一个 helper,命名表达稳定领域概念「渠道侧错误」,与自动禁用/重试口径对齐。实现计划阶段对照 `shouldRetry`/`shouldRetryTaskRelay`/`service.ShouldDisableChannel` 做最终微调并加针对性单测。

### 4.5 出数接口 — 内联进 `GET /api/channel`

- `controller/channel.go` 组装列表分页结果时,收集本页 `channelIds`,调 `channelmetrics.QueryChannelSummary(channelIds, 24h)`,把 `success_rate` / `recent_success_rates` / `latest_bucket_ts` 补进每个 channel 的返回。
- 不直接把展示字段加进可迁移的 `model.Channel` 数据列。优先使用响应 DTO(例如 controller 局部 `channelListItem` 包装 `*model.Channel`);如果因复用必须挂在 `model.Channel` 上,字段必须带 `gorm:"-"` 且仅作响应字段,避免 `AutoMigrate` 修改 `channels` 表。
- 单次批量查询(`WHERE channel_id IN (本页)`),仅本页 N 个渠道,成本低。
- `GET /api/channel/search` 同样处理。
- 未启用(`channel_metrics_setting.Enabled == false`)或无数据时,字段留空,前端不渲染状态条。

### 4.6 配置 — `setting/channel_metrics_setting`

- 独立 `Enabled` 开关(默认与 perf_metrics 一致的默认值)。
- `BucketTime` / `RetentionDays` / `FlushIntervalMinutes` 默认值对齐 `perf_metrics_setting`,独立可调。
- 后端 `channel_metrics_setting` 镜像 `perf_metrics_setting` 的同四个 key: `enabled` / `flush_interval` / `bucket_time`(`minute`|`5min`|`hour`) / `retention_days`。
- 复用现有系统设置持久化/注册模式。前端设置 key 命名 `channel_metrics_setting.{enabled, flush_interval, bucket_time, retention_days}`,UI 加在 `integrations/monitoring-settings-section.tsx` 现有 `perf_metrics_setting` 区块旁(标题类似 `Channel performance metrics`,复用其字段与 `disabled={!enabled}` 联动写法)。
- 默认设置面同步补齐:
  - `web/default/src/features/system-settings/types.ts`
  - `web/default/src/features/system-settings/operations/index.tsx`
  - `web/default/src/features/system-settings/operations/section-registry.tsx`
  - `web/default/src/features/system-settings/integrations/monitoring-settings-section.tsx`
  - `web/default/src/i18n/locales/*.json`

### 4.7 前端 — `ChannelPerfBadge` + 卡片接入

- 新增 `web/default/src/features/channels/components/channel-perf-badge.tsx`: 照 `ModelPerfBadge` 的分段条 + 成功率 % 部分(**去掉**延迟/吞吐两列),复用 `features/performance-metrics/lib/format.ts` 的 `getSuccessRateDotClass` / `getSuccessRateTextClass`。
- `web/default/src/features/channels/types.ts` 的 `channelSchema` 加可选字段: `success_rate?`, `recent_success_rates?: number[]`, `latest_bucket_ts?`。
- `channel-card.tsx`: 在 body(优先级/权重 + 响应/上次测试 grid)与底部分组标签行之间插入 `<ChannelPerfBadge>`(约 `channel-card.tsx:141`)。数据无 → 不渲染。
- 卡片视图必须适配渠道卡片宽度: 不照搬 `ModelPerfBadge` 的固定 `w-[264px]` 和 `<520px` 隐藏策略;渠道版用 `w-full` / `min-w-0` 的紧凑布局,保证窄卡片也能显示 14 格条和百分比。
- `isTagAggregateRow(row.original)` 为真时不渲染 `<ChannelPerfBadge>`。同时 `aggregateChannelsByTag` 创建标签聚合行时要清空 `success_rate` / `recent_success_rates` / `latest_bucket_ts`,避免继承第一个子渠道的指标。
- i18n: 复用现有 `Success rate` 等 key,新增文案走 `web/default/src/i18n/locales/*.json`。
- 经典前端 `web/classic` 不改,新增字段可选、忽略即可。

### 4.8 迁移(跨三库)

- `channel_metrics` 表: `AutoMigrate` 建表,SQLite/MySQL(>=5.7.8)/PostgreSQL(>=9.6) 通吃;唯一/普通索引用 GORM tag,不手写 dialect SQL。
- 保留清理 `DeleteChannelMetricsBefore` 由 `flushLoop` 定期调用。

### 4.9 测试(testify: require 做 setup/致命断言,assert 做取值断言)

- **收集层表驱动**: 记录若干 `(success, latencyMs)` 样本 → `QueryChannelSummary` → 断言 `SuccessRate` / `RecentSuccessRates` 精确值 + 桶边界。
- **普通 relay 口径回归**: 构造重试场景 —— 渠道A(渠道侧错误)失败 → 渠道B成功: 断言 A 记 1 败、B 记 1 成;用户侧 4xx 尝试: 断言不记(分母不变)。
- **Task relay 口径回归**: `RelayTaskSubmit` 成功记成功;非本地渠道侧 task 错误记失败;`LocalError` / 400 用户侧错误不记;重试救回时失败渠道和成功渠道分别计数。
- **`isChannelSideError` 表驱动**: 渠道错误 / SkipRetry / always-skip 码 / 各类 StatusCode → 断言分类。
- **内联接口**: 给定渠道有/无数据,断言列表返回字段存在性与数值。
- **前端标签行回归**: tag mode 聚合行不渲染渠道状态条,且不会继承第一个子渠道的成功率字段。

## 5. 风险与权衡

- **基数**: `channel × bucket × 保留天数`,远小于 `perf_metrics` 的 `model × group × bucket`;可接受。独立 `Enabled` 提供逃生阀。
- **延迟计时**: 按本次尝试,不含重试等待;首版 UI/API 不展示真实流量延迟。失败样本 `latencyMs=0` 只作为内部保留字段,不要用 `RequestCount` 直接当延迟均值分母。
- **Redis 不可用**: 降级为纯内存热桶 + DB(照 `perf_metrics` 现有处理)。
- **记录点侵入 relay**: 普通 relay 和 task relay 的成功/失败分支各加异步记录点,不阻塞;失败分支多一次 `isChannelSideError` 纯函数判定,开销可忽略。
- **经典前端兼容**: 列表接口新增字段可选,经典前端忽略,无破坏。

## 6. 非目标(YAGNI)

- 不做渠道 × 模型下钻(将来需要再加维度)。
- 不在卡片展示真实流量延迟/吞吐。
- 不改动模型 `perf_metrics` 的口径或存储。
- 不做独立的渠道成功率详情图表页(先只卡片一条状态条)。

## 7. 留待实现计划细化

- `isChannelSideError` 对照 `shouldRetry` / `shouldRetryTaskRelay` / `service.ShouldDisableChannel` 的最终判定与边界用例。
- `QueryChannelSummary` 合并 DB 桶 + 内存热桶取「最近 14 桶」的批量查询写法(跨三库)。
- `channel_metrics_setting` 各默认值取值。
- 本次尝试延迟 `attemptLatencyMs` 在重试循环中的准确取点。
- 列表响应 DTO 的落点与 `model.Channel` 复用边界。
