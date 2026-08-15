import { describe, expect, test } from 'vitest'
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

    expect(events.filter((event) => event === 'refresh')).toHaveLength(1)
    expect(events.at(-1)).toBe('refresh')
    expect(
      events.indexOf('update:BepusdtMinTopUp') < events.indexOf('refresh')
    ).toBe(true)
    expect(
      events.includes('update:BepusdtCurrencies') &&
        events.indexOf('update:BepusdtCurrencies') < events.indexOf('refresh')
    ).toBe(true)
    expect(savedValues.get('BepusdtCurrencies')).toBe('USDT,USDC')
    expect(events).not.toContain('update:BepusdtTradeType')
    expect(events).not.toContain('update:BepusdtCashierMode')
  })
})
