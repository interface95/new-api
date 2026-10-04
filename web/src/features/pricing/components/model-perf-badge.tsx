/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { memo } from 'react'
import { useTranslation } from 'react-i18next'

import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import {
  formatLatency,
  formatThroughput,
  getSuccessRateDotClass,
  getSuccessRateTextClass,
} from '@/features/performance-metrics/lib/format'
import type { SuccessRatePoint } from '@/features/performance-metrics/types'
import { cn } from '@/lib/utils'

export type ModelPerfBadgeData = {
  window_start?: number
  window_end?: number
  avg_latency_ms: number
  success_rate: number
  avg_tps: number
  recent_success_series?: SuccessRatePoint[]
  recent_success_rates?: number[]
  recent_bucket_ts?: number[]
  recent_success_counts?: number[]
  recent_failure_counts?: number[]
  latest_bucket_ts?: number
  metric_bucket_seconds?: number
}

export interface ModelPerfBadgeProps extends React.HTMLAttributes<HTMLDivElement> {
  perf: ModelPerfBadgeData | undefined
}

const STATUS_BAR_COUNT = 40

type StatusBar = {
  key: string
  rate: number
  ts?: number
  successCount?: number
  failureCount?: number
}

function formatCompactNumber(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return '—'
  return value > 1 ? String(Math.round(value)) : value.toFixed(1)
}

function formatCompactLatency(ms: number): string {
  if (!Number.isFinite(ms) || ms <= 0) return '—'
  if (ms >= 1_000) return `${formatCompactNumber(ms / 1_000)}s`
  return `${formatCompactNumber(ms)}ms`
}

function formatCompactThroughput(tps: number): string {
  if (!Number.isFinite(tps) || tps <= 0) return '—'
  if (tps >= 1_000) return `${formatCompactNumber(tps / 1_000)}Kt`
  return `${formatCompactNumber(tps)}t`
}

function formatCompactSuccessRate(rate: number): string {
  if (!Number.isFinite(rate)) return '—'
  return `${rate.toFixed(1)}%`
}

function formatBucketTime(ts?: number): string {
  if (!ts || !Number.isFinite(ts)) return ''
  const date = new Date(ts * 1000)
  if (Number.isNaN(date.getTime())) return ''

  const pad = (value: number) => String(value).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(
    date.getDate()
  )} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

function formatBucketClock(ts?: number): string {
  if (!ts || !Number.isFinite(ts)) return ''
  const date = new Date(ts * 1000)
  if (Number.isNaN(date.getTime())) return ''

  const pad = (value: number) => String(value).padStart(2, '0')
  return `${pad(date.getHours())}:${pad(date.getMinutes())}`
}

function formatBucketTimeRange(ts: number, bucketSeconds: number): string {
  const endTs = ts + Math.max(60, bucketSeconds)
  return `${formatBucketClock(ts)} - ${formatBucketClock(endTs)}`
}

function formatBucketTooltip(
  bar: StatusBar,
  bucketSeconds: number,
  labels: { success: string; failed: string }
): string | undefined {
  if (!bar.ts) {
    return undefined
  }
  if (bar.successCount == null || bar.failureCount == null) {
    return `${formatBucketTimeRange(bar.ts, bucketSeconds)}\n${formatCompactSuccessRate(bar.rate)}`
  }
  const successCount = bar.successCount
  const failureCount = bar.failureCount
  return `${formatBucketTimeRange(bar.ts, bucketSeconds)}\n${successCount} ${labels.success} / ${failureCount} ${labels.failed} (${formatCompactSuccessRate(bar.rate)})`
}

// Builds the fixed-length segment list, zipping each real success rate to its
// bucket metadata. Leading capacity is rendered as explicit no-data segments.
function buildStatusBars(
  recentRates: number[],
  recentTs: number[],
  recentSuccessCounts: number[],
  recentFailureCounts: number[]
): StatusBar[] {
  const rates = recentRates.slice(-STATUS_BAR_COUNT)
  if (rates.length === 0) {
    return Array.from(
      { length: STATUS_BAR_COUNT },
      (_, i): StatusBar => ({
        key: `fallback-${i}`,
        rate: Number.NaN,
      })
    )
  }
  const timestamps = recentTs.slice(-STATUS_BAR_COUNT)
  const padCount = Math.max(0, STATUS_BAR_COUNT - rates.length)
  const successCounts = recentSuccessCounts.slice(-STATUS_BAR_COUNT)
  const failureCounts = recentFailureCounts.slice(-STATUS_BAR_COUNT)
  return [
    ...Array.from(
      { length: padCount },
      (_, i): StatusBar => ({
        key: `pad-${i}`,
        rate: Number.NaN,
      })
    ),
    ...rates.map(
      (rate, i): StatusBar => ({
        key: `${timestamps[i] ?? 'missing'}-${rate}-${successCounts[i] ?? 0}-${failureCounts[i] ?? 0}`,
        rate,
        ts: timestamps[i],
        successCount: successCounts[i],
        failureCount: failureCounts[i],
      })
    ),
  ]
}

export const ModelPerfBadge = memo(function ModelPerfBadge(
  props: ModelPerfBadgeProps
) {
  const { t } = useTranslation()

  const success_rate = props.perf?.success_rate ?? Number.NaN
  const avg_latency_ms = props.perf?.avg_latency_ms ?? 0
  const avg_tps = props.perf?.avg_tps ?? 0
  const hasBucketMetrics = props.perf?.recent_success_rates != null
  let statusBars: StatusBar[]
  if (hasBucketMetrics) {
    statusBars = buildStatusBars(
      props.perf?.recent_success_rates ?? [],
      props.perf?.recent_bucket_ts ?? [],
      props.perf?.recent_success_counts ?? [],
      props.perf?.recent_failure_counts ?? []
    )
  } else {
    const windowStart = props.perf?.window_start
    const ratesByHour = new Map(
      (props.perf?.recent_success_series ?? []).map((point) => [
        point.ts,
        point.success_rate,
      ])
    )
    statusBars = Array.from({ length: 24 }, (_, slot) => {
      const ts = windowStart == null ? undefined : windowStart + slot * 3600
      return {
        key: `hour-${slot}`,
        rate: ts == null ? Number.NaN : (ratesByHour.get(ts) ?? Number.NaN),
        ts,
      }
    })
  }
  let latencyText = formatCompactLatency(avg_latency_ms)
  let throughputText = formatCompactThroughput(avg_tps)
  let successRateLabel = formatCompactSuccessRate(success_rate)
  if (!hasBucketMetrics) {
    latencyText = formatLatency(avg_latency_ms)
    throughputText = formatThroughput(avg_tps).replace(' t/s', 't/s')
    if (latencyText === '—') latencyText = '—s'
    if (throughputText === '—') throughputText = '—t/s'
    successRateLabel = Number.isFinite(success_rate)
      ? `${success_rate.toFixed(2)}%`
      : '—'
  }
  const statusHeader = formatBucketTime(props.perf?.latest_bucket_ts)
  const bucketSeconds = hasBucketMetrics
    ? (props.perf?.metric_bucket_seconds ?? 60)
    : 3600
  const tooltipLabels = { success: t('Success'), failed: t('Failed') }

  return (
    <div
      aria-label={t('Performance metrics for the last 24 hours')}
      className={cn(
        'flex w-full min-w-0 flex-wrap items-end justify-end gap-3 tabular-nums',
        props.className
      )}
    >
      <div className='flex min-w-0 grow basis-[278px] flex-col gap-1.5'>
        {/* Top: latency + throughput on the left, timestamp on the right, so the
          success rate can align with the segment rail below. */}
        <div className='flex items-start justify-between gap-x-3'>
          <div className='flex gap-x-3'>
            <div title={t('Average latency')} className='min-w-0'>
              <div className='text-muted-foreground/55 text-[10px] leading-4'>
                {t('Latency short')}
              </div>
              <div className='text-muted-foreground/80 font-mono text-xs leading-4 whitespace-nowrap'>
                {latencyText}
              </div>
            </div>
            <div title={t('Throughput')} className='min-w-0'>
              <div className='text-muted-foreground/55 truncate text-[10px] leading-4'>
                {t('Throughput short')}
              </div>
              <div className='text-muted-foreground/80 font-mono text-xs leading-4 whitespace-nowrap'>
                {throughputText}
              </div>
            </div>
          </div>
          <div
            title={`${t('Success rate')}: ${successRateLabel}`}
            className='min-w-0'
          >
            <span className='text-muted-foreground/55 truncate text-[10px] leading-4'>
              {statusHeader || t('Status')}
            </span>
          </div>
        </div>
        {/* Bottom: thin 40-segment bar spanning the full badge width; each segment
          shows its bucket time on hover. */}
        <div className='flex h-5 items-center gap-2'>
          <TooltipProvider delay={100}>
            <div
              role='img'
              aria-label={t(
                'Recent success-rate samples; gray bars indicate missing data.'
              )}
              className={cn(
                'flex h-5 min-w-0 flex-1 items-center',
                hasBucketMetrics ? 'gap-0.5' : 'gap-px'
              )}
            >
              {statusBars.map((bar) => {
                const tooltip = formatBucketTooltip(
                  bar,
                  bucketSeconds,
                  tooltipLabels
                )
                return (
                  <Tooltip key={bar.key}>
                    <TooltipTrigger
                      render={
                        <span
                          className={cn(
                            'h-5 w-1 rounded-full',
                            Number.isFinite(bar.rate) &&
                              bar.rate >= 0 &&
                              bar.rate <= 100
                              ? getSuccessRateDotClass(bar.rate)
                              : 'bg-muted-foreground/15'
                          )}
                        />
                      }
                    />
                    {tooltip && (
                      <TooltipContent side='top' className='font-mono'>
                        <span className='whitespace-pre-line'>{tooltip}</span>
                      </TooltipContent>
                    )}
                  </Tooltip>
                )
              })}
            </div>
          </TooltipProvider>
          <span
            className={cn(
              'shrink-0 font-mono text-xs leading-4 whitespace-nowrap',
              getSuccessRateTextClass(success_rate)
            )}
          >
            {successRateLabel}
          </span>
        </div>
      </div>
      {props.children}
    </div>
  )
})
