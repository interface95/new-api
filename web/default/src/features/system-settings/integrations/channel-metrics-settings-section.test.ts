import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { describe, test } from 'node:test'

const sectionSource = readFileSync(
  new URL('./channel-metrics-settings-section.tsx', import.meta.url),
  'utf8'
)
const registrySource = readFileSync(
  new URL('../operations/section-registry.tsx', import.meta.url),
  'utf8'
)
const typesSource = readFileSync(new URL('../types.ts', import.meta.url), 'utf8')

describe('ChannelMetricsSettingsSection wiring', () => {
  test('binds all four channel_metrics_setting keys', () => {
    assert.match(sectionSource, /name='channel_metrics_setting\.enabled'/)
    assert.match(sectionSource, /name='channel_metrics_setting\.flush_interval'/)
    assert.match(sectionSource, /name='channel_metrics_setting\.bucket_time'/)
    assert.match(sectionSource, /name='channel_metrics_setting\.retention_days'/)
  })

  test('is a standalone section (no QuotaRemindThreshold)', () => {
    assert.doesNotMatch(sectionSource, /QuotaRemindThreshold/)
  })

  test('registered as its own operations section', () => {
    assert.match(registrySource, /ChannelMetricsSettingsSection/)
    assert.match(registrySource, /id: 'channel-metrics'/)
  })

  test('OperationsSettings type carries the four keys', () => {
    assert.match(typesSource, /'channel_metrics_setting\.enabled': boolean/)
    assert.match(
      typesSource,
      /'channel_metrics_setting\.retention_days': number/
    )
  })
})
