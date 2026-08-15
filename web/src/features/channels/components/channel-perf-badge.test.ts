import { readFileSync } from 'node:fs'
import path from 'node:path'
import { describe, expect, test } from 'vitest'

const badgeSource = readFileSync(
  path.resolve(
    process.cwd(),
    'src/features/channels/components/channel-perf-badge.tsx'
  ),
  'utf8'
)
const cardSource = readFileSync(
  path.resolve(
    process.cwd(),
    'src/features/channels/components/channel-card.tsx'
  ),
  'utf8'
)
const tableSource = readFileSync(
  path.resolve(
    process.cwd(),
    'src/features/channels/components/channels-table.tsx'
  ),
  'utf8'
)
const pricingBadgeSource = readFileSync(
  path.resolve(
    process.cwd(),
    'src/features/pricing/components/model-perf-badge.tsx'
  ),
  'utf8'
)

describe('ChannelPerfBadge wiring', () => {
  test('renders channel segments with the same thickness as model square and a longer rail', () => {
    expect(badgeSource).toMatch(/const STATUS_BAR_COUNT = 80/)
    expect(badgeSource).toMatch(/getSuccessRateDotClass/)
    expect(badgeSource).toMatch(/getSuccessRateTextClass/)
    expect(badgeSource).toMatch(/visibleStatusBars\.map/)
    expect(badgeSource).toMatch(/TooltipContent/)
    expect(badgeSource).toMatch(/formatBucketTimeRange/)
    expect(badgeSource).toMatch(/getVisibleStatusBarCount/)
    expect(badgeSource).toMatch(/ResizeObserver/)
    expect(badgeSource).toMatch(/statusBars\.slice\(-visibleBarCount\)/)
    expect(badgeSource).toMatch(
      /'flex h-5 min-w-0 flex-1 items-center gap-0\.5'/
    )
    expect(badgeSource).not.toMatch(/overflow-hidden pr-2/)
    expect(badgeSource).toMatch(/'h-5 w-1 shrink-0 rounded-full'/)
    expect(badgeSource).not.toMatch(/'h-5 w-1 rounded-full'/)
    expect(badgeSource).not.toMatch(/'h-4 w-0\.5 rounded-full'/)
    expect(badgeSource).toMatch(/formatBucketTooltip/)
    expect(badgeSource).toMatch(/rate: Number\.NaN/)
    expect(badgeSource).not.toMatch(/firstRateTimestamp/)
    expect(badgeSource).toMatch(/successCount/)
    expect(badgeSource).toMatch(/failureCount/)
  })

  test('drops latency/throughput columns (success rate only)', () => {
    expect(badgeSource).not.toMatch(/avg_latency_ms/)
    expect(badgeSource).not.toMatch(/avg_tps/)
  })
})

describe('ModelPerfBadge wiring', () => {
  test('keeps the model square at 40 same-thickness segments', () => {
    expect(pricingBadgeSource).toMatch(/const STATUS_BAR_COUNT = 40/)
    expect(pricingBadgeSource).toMatch(/'h-5 w-1 rounded-full'/)
    expect(pricingBadgeSource).not.toMatch(/'h-4 w-0\.5 rounded-full'/)
  })

  test('uses the same rich bucket tooltip as the channel card', () => {
    expect(pricingBadgeSource).toMatch(/TooltipContent/)
    expect(pricingBadgeSource).toMatch(/formatBucketTimeRange/)
    expect(pricingBadgeSource).toMatch(/formatBucketTooltip/)
    expect(pricingBadgeSource).toMatch(/successCount/)
    expect(pricingBadgeSource).toMatch(/failureCount/)
    expect(pricingBadgeSource).not.toMatch(/title=\{bar\.ts \? formatBucketTime/)
  })

  test('places the success percentage on the same row as the segment rail', () => {
    expect(pricingBadgeSource).not.toMatch(
      /items-baseline gap-x-2[\s\S]*?\{successRateLabel\}[\s\S]*?Bottom: thin 40-segment bar/
    )
    expect(pricingBadgeSource).toMatch(
      /flex h-5 items-center gap-2[\s\S]*?statusBars\.map[\s\S]*?\{successRateLabel\}/
    )
  })
})

describe('channel-card integration', () => {
  test('renders the badge only for non-tag rows with metrics', () => {
    expect(cardSource).toMatch(/import \{ ChannelPerfBadge \}/)
    expect(cardSource).toMatch(/!isTagRow &&/)
    expect(cardSource).toMatch(/recent_success_rates/)
    expect(cardSource).toMatch(/<ChannelPerfBadge/)
  })

  test('keeps cards from stretching into a single long rail on tablet widths', () => {
    expect(tableSource).toMatch(
      /cardGridClassName='grid grid-cols-1 gap-3 sm:gap-4 md:grid-cols-2 xl:grid-cols-3'/
    )
  })
})
