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
import { cn } from '@/lib/utils'
import {
  getSuccessRateDotClass,
  getSuccessRateTextClass,
} from '@/features/performance-metrics/lib/format'

export type ModelPerfBadgeData = {
  avg_latency_ms: number
  success_rate: number
  avg_tps: number
  recent_success_rates?: number[]
  latest_bucket_ts?: number
}

export interface ModelPerfBadgeProps extends React.HTMLAttributes<HTMLDivElement> {
  perf: ModelPerfBadgeData | undefined
}

const STATUS_BAR_COUNT = 14

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

export const ModelPerfBadge = memo(function ModelPerfBadge(
  props: ModelPerfBadgeProps
) {
  const { t } = useTranslation()

  if (!props.perf) {
    return null
  }

  const { avg_latency_ms, avg_tps, success_rate } = props.perf

  const recentRates =
    props.perf.recent_success_rates?.filter((rate) => Number.isFinite(rate)) ??
    []
  const fallbackRate = Number.isFinite(success_rate) ? success_rate : 0
  const statusRates =
    recentRates.length > 0 ? recentRates.slice(-STATUS_BAR_COUNT) : []
  const leadingRate = statusRates[0] ?? fallbackRate
  const statusBars =
    statusRates.length > 0
      ? [
          ...Array(Math.max(0, STATUS_BAR_COUNT - statusRates.length)).fill(
            leadingRate
          ),
          ...statusRates,
        ]
      : Array(STATUS_BAR_COUNT).fill(fallbackRate)
  const successRateLabel = formatCompactSuccessRate(success_rate)
  const statusHeader = formatBucketTime(props.perf.latest_bucket_ts)

  return (
    <div
      className={cn(
        'hidden w-[264px] flex-col gap-1.5 tabular-nums min-[520px]:flex xl:w-[278px]',
        props.className
      )}
    >
      {/* Top: latency + throughput on the left, timestamp + success rate on the
          right (both on the same baseline), so the long bar can span below. */}
      <div className='flex items-start justify-between gap-x-3'>
        <div className='flex gap-x-3'>
          <div title={t('Average latency')} className='min-w-0'>
            <div className='text-muted-foreground/55 text-[10px] leading-4'>
              {t('Latency short')}
            </div>
            <div className='text-muted-foreground/80 font-mono text-xs leading-4 whitespace-nowrap'>
              {formatCompactLatency(avg_latency_ms)}
            </div>
          </div>
          <div title={t('Throughput')} className='min-w-0'>
            <div className='text-muted-foreground/55 truncate text-[10px] leading-4'>
              {t('Throughput short')}
            </div>
            <div className='text-muted-foreground/80 font-mono text-xs leading-4 whitespace-nowrap'>
              {formatCompactThroughput(avg_tps)}
            </div>
          </div>
        </div>
        <div
          title={`${t('Success rate')}: ${successRateLabel}`}
          className='flex min-w-0 items-baseline gap-x-2'
        >
          <span className='text-muted-foreground/55 truncate text-[10px] leading-4'>
            {statusHeader || t('Status short')}
          </span>
          <span
            className={cn(
              'font-mono text-xs leading-4 whitespace-nowrap',
              getSuccessRateTextClass(success_rate)
            )}
          >
            {successRateLabel}
          </span>
        </div>
      </div>
      {/* Bottom: 14-segment bar spanning the full badge width. */}
      <div className='flex h-4 items-center gap-0.5'>
        {statusBars.map((rate, index) => (
          <span
            key={`${index}-${rate}`}
            className={cn(
              'h-4 flex-1 rounded-full',
              getSuccessRateDotClass(rate)
            )}
          />
        ))}
      </div>
    </div>
  )
})
