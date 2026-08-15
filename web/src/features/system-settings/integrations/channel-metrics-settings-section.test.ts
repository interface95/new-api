import { readFileSync } from 'node:fs'
import path from 'node:path'
import { describe, expect, test } from 'vitest'

const sectionSource = readFileSync(
  path.resolve(
    process.cwd(),
    'src/features/system-settings/integrations/channel-metrics-settings-section.tsx'
  ),
  'utf8'
)
const registrySource = readFileSync(
  path.resolve(
    process.cwd(),
    'src/features/system-settings/operations/section-registry.tsx'
  ),
  'utf8'
)
const typesSource = readFileSync(
  path.resolve(process.cwd(), 'src/features/system-settings/types.ts'),
  'utf8'
)

describe('ChannelMetricsSettingsSection wiring', () => {
  test('binds all four channel_metrics_setting keys', () => {
    expect(sectionSource).toMatch(/name='channel_metrics_setting\.enabled'/)
    expect(sectionSource).toMatch(
      /name='channel_metrics_setting\.flush_interval'/
    )
    expect(sectionSource).toMatch(
      /name='channel_metrics_setting\.bucket_time'/
    )
    expect(sectionSource).toMatch(
      /name='channel_metrics_setting\.retention_days'/
    )
  })

  test('is a standalone section (no QuotaRemindThreshold)', () => {
    expect(sectionSource).not.toMatch(/QuotaRemindThreshold/)
  })

  test('registered as its own operations section', () => {
    expect(registrySource).toMatch(/ChannelMetricsSettingsSection/)
    expect(registrySource).toMatch(/id: 'channel-metrics'/)
  })

  test('OperationsSettings type carries the four keys', () => {
    expect(typesSource).toMatch(/'channel_metrics_setting\.enabled': boolean/)
    expect(typesSource).toMatch(
      /'channel_metrics_setting\.retention_days': number/
    )
  })
})
