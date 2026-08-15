import i18n from 'i18next'
import { renderToStaticMarkup } from 'react-dom/server'
import { initReactI18next } from 'react-i18next'
import { beforeAll, describe, expect, test } from 'vitest'

import type { PresetAmount, TopupInfo } from '../types'
import { RechargeFormCard } from './recharge-form-card'

const topupInfoBase: TopupInfo = {
  enable_online_topup: false,
  enable_stripe_topup: false,
  pay_methods: [],
  min_topup: 1,
  stripe_min_topup: 1,
  amount_options: [],
  discount: {},
  enable_redemption: true,
  payment_compliance_confirmed: true,
  payment_compliance_terms_version: 'v1',
}

const noop = () => {}
const presetAmounts: PresetAmount[] = []

beforeAll(async () => {
  if (i18n.isInitialized) return

  await i18n.use(initReactI18next).init({
    lng: 'en',
    fallbackLng: 'en',
    resources: { en: { translation: {} } },
    interpolation: { escapeValue: false },
  })
})

function renderRechargeForm(topupInfo: TopupInfo) {
  return renderToStaticMarkup(
    <RechargeFormCard
      topupInfo={topupInfo}
      presetAmounts={presetAmounts}
      selectedPreset={null}
      onSelectPreset={noop}
      topupAmount={50}
      onTopupAmountChange={noop}
      paymentAmount={50}
      calculating={false}
      onPaymentMethodSelect={noop}
      paymentLoading={null}
      redemptionCode=''
      onRedemptionCodeChange={noop}
      onRedeem={noop}
      redeeming={false}
      loading={false}
    />
  )
}

describe('RechargeFormCard topup availability', () => {
  test('shows the topup form when BEpusdt is the only enabled online payment', () => {
    const html = renderRechargeForm({
      ...topupInfoBase,
      enable_bepusdt_topup: true,
      bepusdt_min_topup: 1,
      pay_methods: [
        {
          name: 'USDT',
          type: 'bepusdt',
          min_topup: 1,
        },
      ],
    })

    expect(html).not.toMatch(/Online topup is not enabled/)
    expect(html).toMatch(/Payment Method/)
    expect(html).toMatch(/USDT/)
  })
})
