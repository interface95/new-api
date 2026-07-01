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

export type ChannelPerfBadgeData = {
  success_rate: number
  recent_success_rates?: number[]
  latest_bucket_ts?: number
}

export interface ChannelPerfBadgeProps
  extends React.HTMLAttributes<HTMLDivElement> {
  perf: ChannelPerfBadgeData | undefined
}

const STATUS_BAR_COUNT = 14

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

/**
 * Channel success-rate status bar: 14 segmented pills coloured by each recent
 * bucket's success rate, plus the overall rate. Lifted from ModelPerfBadge's
 * status column but drops the latency/throughput columns and the fixed width /
 * hide-below-520px behaviour so it fits the (narrower) channel card.
 */
export const ChannelPerfBadge = memo(function ChannelPerfBadge(
  props: ChannelPerfBadgeProps
) {
  const { t } = useTranslation()

  if (!props.perf) {
    return null
  }

  const { success_rate } = props.perf
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
      className={cn('flex w-full min-w-0 flex-col gap-0.5', props.className)}
      title={`${t('Success rate')}: ${successRateLabel}`}
    >
      <div className='text-muted-foreground/55 truncate text-[10px] leading-4'>
        {statusHeader || t('Status short')}
      </div>
      <div className='flex h-5 items-center gap-2'>
        <div className='flex h-4 min-w-0 flex-1 items-center gap-0.5'>
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
