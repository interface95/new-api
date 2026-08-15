import { readFileSync } from 'node:fs'
import path from 'node:path'
import { describe, expect, test } from 'vitest'

const source = readFileSync(
  path.resolve(
    process.cwd(),
    'src/features/system-settings/integrations/payment-settings-section.tsx'
  ),
  'utf8'
)

describe('PaymentSettingsSection source wiring', () => {
  test('keeps the Epay enable option wired into the form and saves', () => {
    expect(source).toMatch(/EpayEnabled:\s*z\.boolean\(\)/)
    expect(source).toMatch(/name='EpayEnabled'/)
    expect(source).toMatch(/key:\s*'EpayEnabled'/)
    expect(source).toMatch(/Enable Epay gateway/)
  })
})
