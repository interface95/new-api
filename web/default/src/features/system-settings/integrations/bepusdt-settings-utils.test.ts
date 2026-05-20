import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import {
  buildBepusdtSettingsUpdates,
  saveBepusdtSettingsBatch,
} from './bepusdt-settings-utils'

describe('BEpusdt settings persistence', () => {
  test('refreshes system options only after all BEpusdt updates finish', async () => {
    const updates = buildBepusdtSettingsUpdates({
      BepusdtEnabled: true,
      BepusdtGatewayURL: 'https://pay.example.com/',
      BepusdtAuthToken: '',
      BepusdtTradeType: 'usdt.trc20',
      BepusdtFiat: 'CNY',
      BepusdtReturnURL: '',
      BepusdtUnitPrice: 1,
      BepusdtMinTopUp: 50,
    })
    const events: string[] = []

    await saveBepusdtSettingsBatch(
      updates,
      async (option) => {
        events.push(`update:${option.key}`)
      },
      async () => {
        events.push('refresh')
      }
    )

    assert.equal(events.filter((event) => event === 'refresh').length, 1)
    assert.equal(events.at(-1), 'refresh')
    assert.ok(
      events.indexOf('update:BepusdtMinTopUp') < events.indexOf('refresh')
    )
  })
})
