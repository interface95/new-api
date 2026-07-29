/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import i18next from 'i18next'
import { useState, useCallback } from 'react'
import { toast } from 'sonner'

import {
  calculateAmount,
  calculateBepusdtAmount,
  calculateStripeAmount,
  calculateWaffoAmount,
  calculateWaffoPancakeAmount,
  requestBepusdtPayment,
  requestPayment,
  requestStripePayment,
  isApiSuccess,
} from '../api'
import {
  isBepusdtPayment,
  isStripePayment,
  isWaffoPayment,
  isWaffoPancakePayment,
  submitPaymentForm,
} from '../lib'
import type {
  AmountRequest,
  AmountResponse,
  BepusdtPaymentResponse,
  PaymentResponse,
  StripePaymentResponse,
} from '../types'

// ============================================================================
// Payment Hook
// ============================================================================

type AmountCalculator = (request: AmountRequest) => Promise<AmountResponse>

export interface PaymentAmountCalculators {
  regular: AmountCalculator
  bepusdt: AmountCalculator
  stripe: AmountCalculator
  waffo: AmountCalculator
  waffoPancake: AmountCalculator
}

const defaultPaymentAmountCalculators: PaymentAmountCalculators = {
  regular: calculateAmount,
  bepusdt: calculateBepusdtAmount,
  stripe: calculateStripeAmount,
  waffo: calculateWaffoAmount,
  waffoPancake: calculateWaffoPancakeAmount,
}

export async function requestPaymentAmount(
  topupAmount: number,
  paymentType: string,
  calculators: PaymentAmountCalculators = defaultPaymentAmountCalculators
): Promise<number> {
  let calculator = calculators.regular
  if (isStripePayment(paymentType)) {
    calculator = calculators.stripe
  } else if (isBepusdtPayment(paymentType)) {
    calculator = calculators.bepusdt
  } else if (isWaffoPayment(paymentType)) {
    calculator = calculators.waffo
  } else if (isWaffoPancakePayment(paymentType)) {
    calculator = calculators.waffoPancake
  }

  const response = await calculator({ amount: topupAmount })
  if (!isApiSuccess(response) || !response.data) {
    return 0
  }

  return Number.parseFloat(response.data)
}

function getCheckoutUrl(data: unknown): string | null {
  if (!data || typeof data !== 'object') {
    return null
  }
  if ('checkout_url' in data && typeof data.checkout_url === 'string') {
    return data.checkout_url
  }
  if ('payment_url' in data && typeof data.payment_url === 'string') {
    return data.payment_url
  }
  return null
}

function isSafeHttpCheckoutUrl(value: string): boolean {
  try {
    const url = new URL(value.trim())
    return url.protocol === 'http:' || url.protocol === 'https:'
  } catch {
    return false
  }
}

export function usePayment() {
  const [amount, setAmount] = useState<number>(0)
  const [calculating, setCalculating] = useState(false)
  const [processing, setProcessing] = useState(false)

  // Calculate payment amount
  const calculatePaymentAmount = useCallback(
    async (topupAmount: number, paymentType: string) => {
      try {
        setCalculating(true)
        const calculatedAmount = await requestPaymentAmount(
          topupAmount,
          paymentType
        )
        setAmount(calculatedAmount)
        return calculatedAmount
      } catch {
        setAmount(0)
        return 0
      } finally {
        setCalculating(false)
      }
    },
    []
  )

  // Process payment
  const processPayment = useCallback(
    async (topupAmount: number, paymentType: string) => {
      try {
        setProcessing(true)

        const isStripe = isStripePayment(paymentType)
        const isBepusdt = isBepusdtPayment(paymentType)
        const amount = Math.floor(topupAmount)

        let response:
          | BepusdtPaymentResponse
          | PaymentResponse
          | StripePaymentResponse
        if (isStripe) {
          response = await requestStripePayment({
            amount,
            payment_method: 'stripe',
          })
        } else if (isBepusdt) {
          response = await requestBepusdtPayment({ amount })
        } else {
          response = await requestPayment({
            amount,
            payment_method: paymentType,
          })
        }

        if (!isApiSuccess(response)) {
          toast.error(response.message || i18next.t('Payment request failed'))
          return false
        }

        // Handle Stripe payment
        if (isStripe && response.data && typeof response.data === 'object') {
          const payLink =
            'pay_link' in response.data ? response.data.pay_link : null
          if (typeof payLink !== 'string') {
            return false
          }
          window.open(payLink, '_blank')
          toast.success(i18next.t('Redirecting to payment page...'))
          return true
        }

        if (isBepusdt) {
          const checkoutUrl = getCheckoutUrl(response.data)
          if (!checkoutUrl) {
            return false
          }
          if (!isSafeHttpCheckoutUrl(checkoutUrl)) {
            toast.error(i18next.t('Invalid payment redirect URL'))
            return false
          }
          window.open(checkoutUrl, '_blank', 'noopener,noreferrer')
          toast.success(i18next.t('Redirecting to BEpusdt payment page...'))
          return true
        }

        // Handle non-Stripe payment
        if (
          !isStripe &&
          !isBepusdt &&
          response.data &&
          typeof response.data === 'object'
        ) {
          const url = (response as unknown as { url?: string }).url
          if (url) {
            submitPaymentForm(url, response.data)
            toast.success(i18next.t('Redirecting to payment page...'))
            return true
          }
        }

        return false
      } catch {
        toast.error(i18next.t('Payment request failed'))
        return false
      } finally {
        setProcessing(false)
      }
    },
    []
  )

  return {
    amount,
    calculating,
    processing,
    calculatePaymentAmount,
    processPayment,
    setAmount,
  }
}
