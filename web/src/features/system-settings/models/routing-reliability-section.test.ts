import { readFileSync } from 'node:fs'
import path from 'node:path'
import { describe, expect, test } from 'vitest'

const sectionSource = readFileSync(
  path.resolve(
    process.cwd(),
    'src/features/system-settings/models/routing-reliability-section.tsx'
  ),
  'utf8'
)

const registrySource = readFileSync(
  path.resolve(
    process.cwd(),
    'src/features/system-settings/models/section-registry.tsx'
  ),
  'utf8'
)

const typesSource = readFileSync(
  path.resolve(process.cwd(), 'src/features/system-settings/types.ts'),
  'utf8'
)

describe('RoutingReliabilitySection source wiring', () => {
  test('keeps auto-disable consecutive failure settings wired into the form and saves', () => {
    expect(sectionSource).toMatch(
      /AutomaticDisableFailureThreshold:\s*z\.coerce/
    )
    expect(sectionSource).toMatch(
      /AutomaticDisableFailureWindowSeconds:\s*z\.coerce/
    )
    expect(sectionSource).toMatch(/name='AutomaticDisableFailureThreshold'/)
    expect(sectionSource).toMatch(
      /name='AutomaticDisableFailureWindowSeconds'/
    )
    expect(sectionSource).toMatch(/Consecutive failure threshold/)
    expect(sectionSource).toMatch(/Failure counter TTL \(seconds\)/)
    expect(typesSource).toMatch(/AutomaticDisableFailureThreshold:\s*number/)
    expect(typesSource).toMatch(
      /AutomaticDisableFailureWindowSeconds:\s*number/
    )
    expect(registrySource).toMatch(
      /settings\.AutomaticDisableFailureThreshold/
    )
    expect(registrySource).toMatch(
      /settings\.AutomaticDisableFailureWindowSeconds/
    )
  })
})
