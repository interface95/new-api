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

describe('ChannelPerfBadge wiring', () => {
  test('renders 14 segments coloured by the shared success-rate helper', () => {
    assert.match(badgeSource, /const STATUS_BAR_COUNT = 14/)
    assert.match(badgeSource, /getSuccessRateDotClass/)
    assert.match(badgeSource, /getSuccessRateTextClass/)
    assert.match(badgeSource, /statusBars\.map/)
  })

  test('drops latency/throughput columns (success rate only)', () => {
    assert.doesNotMatch(badgeSource, /avg_latency_ms/)
    assert.doesNotMatch(badgeSource, /avg_tps/)
  })
})

describe('channel-card integration', () => {
  test('renders the badge only for non-tag rows with metrics', () => {
    assert.match(cardSource, /import \{ ChannelPerfBadge \}/)
    assert.match(cardSource, /!isTagRow &&/)
    assert.match(cardSource, /recent_success_rates/)
    assert.match(cardSource, /<ChannelPerfBadge/)
  })
})
