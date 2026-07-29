import type { UpdateOptionRequest } from '../types'
import type { BepusdtSettingsValues } from './bepusdt-settings-section'
import { removeTrailingSlash } from './utils'

export type BepusdtSettingsUpdate = UpdateOptionRequest & {
  value: string
}

function normalizeBepusdtCurrencies(value: string): string {
  const seen = new Set<string>()
  return value
    .split(',')
    .map((currency) => currency.trim().toUpperCase())
    .filter((currency) => {
      if (!currency || seen.has(currency)) {
        return false
      }
      seen.add(currency)
      return true
    })
    .join(',')
}

export function buildBepusdtSettingsUpdates(
  values: BepusdtSettingsValues
): BepusdtSettingsUpdate[] {
  const updates: BepusdtSettingsUpdate[] = [
    { key: 'BepusdtEnabled', value: values.BepusdtEnabled ? 'true' : 'false' },
    {
      key: 'BepusdtGatewayURL',
      value: removeTrailingSlash(values.BepusdtGatewayURL || ''),
    },
    {
      key: 'BepusdtCurrencies',
      value: normalizeBepusdtCurrencies(values.BepusdtCurrencies || ''),
    },
    { key: 'BepusdtFiat', value: values.BepusdtFiat || 'CNY' },
    {
      key: 'BepusdtReturnURL',
      value: removeTrailingSlash(values.BepusdtReturnURL || ''),
    },
    {
      key: 'BepusdtUnitPrice',
      value: String(values.BepusdtUnitPrice ?? 1),
    },
    { key: 'BepusdtMinTopUp', value: String(values.BepusdtMinTopUp ?? 1) },
  ]

  const authToken = (values.BepusdtAuthToken || '').trim()
  if (authToken) {
    updates.push({ key: 'BepusdtAuthToken', value: authToken })
  }

  return updates
}

export async function saveBepusdtSettingsBatch(
  updates: BepusdtSettingsUpdate[],
  updateOption: (option: BepusdtSettingsUpdate) => Promise<void>,
  refreshOptions: () => Promise<void>
): Promise<void> {
  for (const option of updates) {
    await updateOption(option)
  }
  await refreshOptions()
}
