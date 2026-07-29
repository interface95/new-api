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
import { memo, useLayoutEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import {
  getSuccessRateDotClass,
  getSuccessRateTextClass,
} from '@/features/performance-metrics/lib/format'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'

export type ChannelPerfBadgeData = {
  success_rate: number
  recent_success_rates?: number[]
  recent_bucket_ts?: number[]
  recent_success_counts?: number[]
  recent_failure_counts?: number[]
  latest_bucket_ts?: number
  metric_bucket_seconds?: number
}

export interface ChannelPerfBadgeProps
  extends React.HTMLAttributes<HTMLDivElement> {
  perf: ChannelPerfBadgeData | undefined
}

const STATUS_BAR_COUNT = 80
const STATUS_BAR_WIDTH_PX = 4
const STATUS_BAR_GAP_PX = 2

type StatusBar = {
  key: string
  rate: number
  ts?: number
  successCount?: number
  failureCount?: number
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
  const successCount = bar.successCount ?? 0
  const failureCount = bar.failureCount ?? 0
  return `${formatBucketTimeRange(bar.ts, bucketSeconds)}\n${successCount} ${labels.success} / ${failureCount} ${labels.failed} (${formatCompactSuccessRate(bar.rate)})`
}

function getVisibleStatusBarCount(width: number): number {
  if (!Number.isFinite(width) || width <= 0) {
    return STATUS_BAR_COUNT
  }

  return Math.max(
    1,
    Math.min(
      STATUS_BAR_COUNT,
      Math.floor(
        (width + STATUS_BAR_GAP_PX) /
          (STATUS_BAR_WIDTH_PX + STATUS_BAR_GAP_PX)
      )
    )
  )
}

function useVisibleStatusBarCount() {
  const railRef = useRef<HTMLDivElement>(null)
  const [visibleBarCount, setVisibleBarCount] = useState(STATUS_BAR_COUNT)

  useLayoutEffect(() => {
    const rail = railRef.current
    if (!rail || typeof ResizeObserver === 'undefined') {
      return
    }

    const updateVisibleBarCount = (width: number) => {
      setVisibleBarCount(getVisibleStatusBarCount(width))
    }

    updateVisibleBarCount(rail.clientWidth)

    const observer = new ResizeObserver((entries) => {
      updateVisibleBarCount(entries[0]?.contentRect.width ?? rail.clientWidth)
    })
    observer.observe(rail)

    return () => observer.disconnect()
  }, [])

  return { railRef, visibleBarCount }
}

// Builds the fixed-length segment list, right-aligning bucket timestamps/counts
// to rates and inferring missing leading timestamps from the bucket interval.
function buildStatusBars(
  recentRates: number[],
  recentTs: number[],
  recentSuccessCounts: number[],
  recentFailureCounts: number[],
  fallbackRate: number,
  bucketSeconds: number
): StatusBar[] {
  const rates = recentRates.slice(-STATUS_BAR_COUNT)
  if (rates.length === 0) {
    return Array.from(
      { length: STATUS_BAR_COUNT },
      (_, i): StatusBar => ({
        key: `fallback-${i}-${fallbackRate}`,
        rate: fallbackRate,
      })
    )
  }
  const timestamps = recentTs.slice(-rates.length)
  const timestampOffset = Math.max(0, rates.length - timestamps.length)
  const firstTimestamp = timestamps[0]
  const leadingRate = rates[0] ?? fallbackRate
  const padCount = Math.max(0, STATUS_BAR_COUNT - rates.length)
  const successCounts = recentSuccessCounts.slice(-rates.length)
  const successCountOffset = Math.max(0, rates.length - successCounts.length)
  const failureCounts = recentFailureCounts.slice(-rates.length)
  const failureCountOffset = Math.max(0, rates.length - failureCounts.length)
  const getTimestamp = (rateIndex: number): number | undefined => {
    if (rateIndex >= timestampOffset) {
      return timestamps[rateIndex - timestampOffset]
    }
    return Number.isFinite(firstTimestamp)
      ? firstTimestamp - (timestampOffset - rateIndex) * bucketSeconds
      : undefined
  }
  const firstRateTimestamp = getTimestamp(0)
  const hasFirstRateTimestamp =
    typeof firstRateTimestamp === 'number' && Number.isFinite(firstRateTimestamp)
  return [
    ...Array.from(
      { length: padCount },
      (_, i): StatusBar => ({
        key: `pad-${i}-${leadingRate}`,
        rate: leadingRate,
        ts: hasFirstRateTimestamp
          ? firstRateTimestamp - (padCount - i) * bucketSeconds
          : undefined,
      })
    ),
    ...rates.map((rate, i): StatusBar => ({
      key: `${getTimestamp(i) ?? 'missing'}-${rate}-${successCounts[i - successCountOffset] ?? 0}-${failureCounts[i - failureCountOffset] ?? 0}`,
      rate,
      ts: getTimestamp(i),
      successCount: successCounts[i - successCountOffset],
      failureCount: failureCounts[i - failureCountOffset],
    })),
  ]
}

/**
 * Channel success-rate status bar: 80 segmented pills coloured by each
 * recent bucket's success rate, each showing its bucket time on hover, plus the
 * overall rate. Lifted from ModelPerfBadge but drops the latency/throughput
 * columns and the fixed width / hide-below-520px behaviour so it fits the
 * (narrower) channel card.
 */
export const ChannelPerfBadge = memo(function ChannelPerfBadge(
  props: ChannelPerfBadgeProps
) {
  const { t } = useTranslation()
  const { railRef, visibleBarCount } = useVisibleStatusBarCount()

  if (!props.perf) {
    return null
  }

  const { success_rate } = props.perf
  const recentRates =
    props.perf.recent_success_rates?.filter((rate) => Number.isFinite(rate)) ??
    []
  const fallbackRate = Number.isFinite(success_rate) ? success_rate : 0
  const bucketSeconds = props.perf.metric_bucket_seconds ?? 60
  const statusBars = buildStatusBars(
    recentRates,
    props.perf.recent_bucket_ts ?? [],
    props.perf.recent_success_counts ?? [],
    props.perf.recent_failure_counts ?? [],
    fallbackRate,
    bucketSeconds
  )
  const visibleStatusBars = statusBars.slice(-visibleBarCount)
  const successRateLabel = formatCompactSuccessRate(success_rate)
  const statusHeader = formatBucketTime(props.perf.latest_bucket_ts)
  const tooltipLabels = { success: t('Success'), failed: t('Failed') }

  return (
    <div
      className={cn('flex w-full min-w-0 flex-col gap-0.5', props.className)}
      title={`${t('Success rate')}: ${successRateLabel}`}
    >
      <div className='text-muted-foreground/55 truncate text-[10px] leading-4'>
        {statusHeader || t('Status short')}
      </div>
      <div className='flex h-5 items-center gap-2'>
        <TooltipProvider delay={100}>
          <div
            ref={railRef}
            className='flex h-5 min-w-0 flex-1 items-center gap-0.5'
          >
            {visibleStatusBars.map((bar) => {
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
                          'h-5 w-1 shrink-0 rounded-full',
                          getSuccessRateDotClass(bar.rate)
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
        <div
          className={cn(
            'min-w-10 shrink-0 text-right font-mono text-xs leading-4 whitespace-nowrap',
            getSuccessRateTextClass(success_rate)
          )}
        >
          {successRateLabel}
        </div>
      </div>
    </div>
  )
})
