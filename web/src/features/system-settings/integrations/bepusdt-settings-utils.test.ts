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
      BepusdtCurrencies: 'usdt, USDC , usdt',
      BepusdtFiat: 'CNY',
      BepusdtReturnURL: '',
      BepusdtUnitPrice: 1,
      BepusdtMinTopUp: 50,
    })
    const events: string[] = []
    const savedValues = new Map<string, string>()

    await saveBepusdtSettingsBatch(
      updates,
      async (option) => {
        events.push(`update:${option.key}`)
        savedValues.set(option.key, option.value)
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
    assert.ok(
      events.includes('update:BepusdtCurrencies') &&
      events.indexOf('update:BepusdtCurrencies') < events.indexOf('refresh')
    )
    assert.equal(savedValues.get('BepusdtCurrencies'), 'USDT,USDC')
    assert.ok(!events.includes('update:BepusdtTradeType'))
    assert.ok(!events.includes('update:BepusdtCashierMode'))
  })
})
