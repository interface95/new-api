import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { describe, test } from 'node:test'

const source = readFileSync(
  new URL('./payment-settings-section.tsx', import.meta.url),
  'utf8'
)

describe('PaymentSettingsSection source wiring', () => {
  test('keeps the Epay enable option wired into the form and saves', () => {
    assert.match(source, /EpayEnabled:\s*z\.boolean\(\)/)
    assert.match(source, /name='EpayEnabled'/)
    assert.match(source, /key:\s*'EpayEnabled'/)
    assert.match(source, /Enable Epay gateway/)
  })
})
