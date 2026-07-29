import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { describe, test } from 'node:test'

const badgeSource = readFileSync(
  new URL('./channel-perf-badge.tsx', import.meta.url),
  'utf8'
)
const cardSource = readFileSync(
  new URL('./channel-card.tsx', import.meta.url),
  'utf8'
)
const tableSource = readFileSync(
  new URL('./channels-table.tsx', import.meta.url),
  'utf8'
)
const pricingBadgeSource = readFileSync(
  new URL('../../pricing/components/model-perf-badge.tsx', import.meta.url),
  'utf8'
)

describe('ChannelPerfBadge wiring', () => {
  test('renders channel segments with the same thickness as model square and a longer rail', () => {
    assert.match(badgeSource, /const STATUS_BAR_COUNT = 80/)
    assert.match(badgeSource, /getSuccessRateDotClass/)
    assert.match(badgeSource, /getSuccessRateTextClass/)
    assert.match(badgeSource, /visibleStatusBars\.map/)
    assert.match(badgeSource, /TooltipContent/)
    assert.match(badgeSource, /formatBucketTimeRange/)
    assert.match(badgeSource, /getVisibleStatusBarCount/)
    assert.match(badgeSource, /ResizeObserver/)
    assert.match(badgeSource, /statusBars\.slice\(-visibleBarCount\)/)
    assert.match(
      badgeSource,
      /'flex h-5 min-w-0 flex-1 items-center gap-0\.5'/
    )
    assert.doesNotMatch(badgeSource, /overflow-hidden pr-2/)
    assert.match(badgeSource, /'h-5 w-1 shrink-0 rounded-full'/)
    assert.doesNotMatch(badgeSource, /'h-5 w-1 rounded-full'/)
    assert.doesNotMatch(badgeSource, /'h-4 w-0\.5 rounded-full'/)
    assert.match(badgeSource, /formatBucketTooltip/)
    assert.match(badgeSource, /const timestampOffset = Math\.max/)
    assert.match(
      badgeSource,
      /firstRateTimestamp - \(padCount - i\) \* bucketSeconds/
    )
    assert.match(badgeSource, /successCount/)
    assert.match(badgeSource, /failureCount/)
  })

  test('drops latency/throughput columns (success rate only)', () => {
    assert.doesNotMatch(badgeSource, /avg_latency_ms/)
    assert.doesNotMatch(badgeSource, /avg_tps/)
  })
})

describe('ModelPerfBadge wiring', () => {
  test('keeps the model square at 40 same-thickness segments', () => {
    assert.match(pricingBadgeSource, /const STATUS_BAR_COUNT = 40/)
    assert.match(pricingBadgeSource, /'h-5 w-1 rounded-full'/)
    assert.doesNotMatch(pricingBadgeSource, /'h-4 w-0\.5 rounded-full'/)
  })

  test('uses the same rich bucket tooltip as the channel card', () => {
    assert.match(pricingBadgeSource, /TooltipContent/)
    assert.match(pricingBadgeSource, /formatBucketTimeRange/)
    assert.match(pricingBadgeSource, /formatBucketTooltip/)
    assert.match(pricingBadgeSource, /successCount/)
    assert.match(pricingBadgeSource, /failureCount/)
    assert.doesNotMatch(pricingBadgeSource, /title=\{bar\.ts \? formatBucketTime/)
  })

  test('places the success percentage on the same row as the segment rail', () => {
    assert.doesNotMatch(
      pricingBadgeSource,
      /items-baseline gap-x-2[\s\S]*?\{successRateLabel\}[\s\S]*?Bottom: thin 40-segment bar/
    )
    assert.match(
      pricingBadgeSource,
      /flex h-5 items-center gap-2[\s\S]*?statusBars\.map[\s\S]*?\{successRateLabel\}/
    )
  })
})

describe('channel-card integration', () => {
  test('renders the badge only for non-tag rows with metrics', () => {
    assert.match(cardSource, /import \{ ChannelPerfBadge \}/)
    assert.match(cardSource, /!isTagRow &&/)
    assert.match(cardSource, /recent_success_rates/)
    assert.match(cardSource, /<ChannelPerfBadge/)
  })

  test('keeps cards from stretching into a single long rail on tablet widths', () => {
    assert.match(
      tableSource,
      /cardGridClassName='grid grid-cols-1 gap-3 sm:gap-4 md:grid-cols-2 xl:grid-cols-3'/
    )
  })
})
