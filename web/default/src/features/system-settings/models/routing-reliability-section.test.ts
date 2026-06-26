import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { describe, test } from 'node:test'

const sectionSource = readFileSync(
  new URL('./routing-reliability-section.tsx', import.meta.url),
  'utf8'
)

const registrySource = readFileSync(
  new URL('./section-registry.tsx', import.meta.url),
  'utf8'
)

const typesSource = readFileSync(
  new URL('../types.ts', import.meta.url),
  'utf8'
)

describe('RoutingReliabilitySection source wiring', () => {
  test('keeps auto-disable consecutive failure settings wired into the form and saves', () => {
    assert.match(sectionSource, /AutomaticDisableFailureThreshold:\s*z\.coerce/)
    assert.match(sectionSource, /AutomaticDisableFailureWindowSeconds:\s*z\.coerce/)
    assert.match(sectionSource, /name='AutomaticDisableFailureThreshold'/)
    assert.match(sectionSource, /name='AutomaticDisableFailureWindowSeconds'/)
    assert.match(sectionSource, /Consecutive failure threshold/)
    assert.match(sectionSource, /Failure counter TTL \(seconds\)/)
    assert.match(typesSource, /AutomaticDisableFailureThreshold:\s*number/)
    assert.match(typesSource, /AutomaticDisableFailureWindowSeconds:\s*number/)
    assert.match(registrySource, /settings\.AutomaticDisableFailureThreshold/)
    assert.match(registrySource, /settings\.AutomaticDisableFailureWindowSeconds/)
  })
})
