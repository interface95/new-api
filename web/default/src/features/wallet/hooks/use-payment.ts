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
import { useState, useCallback } from 'react'
import i18next from 'i18next'
import { toast } from 'sonner'
import {
  calculateAmount,
  calculateBepusdtAmount,
  calculateStripeAmount,
  calculateWaffoPancakeAmount,
  requestBepusdtPayment,
  requestPayment,
  requestStripePayment,
  isApiSuccess,
} from '../api'
import {
  isStripePayment,
  isBepusdtPayment,
  isWaffoPancakePayment,
  submitPaymentForm,
} from '../lib'
import type {
  AmountResponse,
  BepusdtPaymentResponse,
  PaymentResponse,
  StripePaymentResponse,
} from '../types'

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
  const trimmed = value.trim()
  if (!trimmed) {
    return false
  }

  try {
    const url = new URL(trimmed)
    return url.protocol === 'http:' || url.protocol === 'https:'
  } catch {
    return false
  }
}

// ============================================================================
// Payment Hook
// ============================================================================

export function usePayment() {
  const [amount, setAmount] = useState<number>(0)
  const [calculating, setCalculating] = useState(false)
  const [processing, setProcessing] = useState(false)

  // Calculate payment amount
  const calculatePaymentAmount = useCallback(
    async (topupAmount: number, paymentType: string) => {
      try {
        setCalculating(true)

        const isStripe = isStripePayment(paymentType)
        const isPancake = isWaffoPancakePayment(paymentType)
        const isBepusdt = isBepusdtPayment(paymentType)

        let response: AmountResponse
        if (isStripe) {
          response = await calculateStripeAmount({ amount: topupAmount })
        } else if (isPancake) {
          response = await calculateWaffoPancakeAmount({ amount: topupAmount })
        } else if (isBepusdt) {
          response = await calculateBepusdtAmount({ amount: topupAmount })
        } else {
          response = await calculateAmount({ amount: topupAmount })
        }

        if (isApiSuccess(response) && response.data) {
          const calculatedAmount = parseFloat(response.data)
          setAmount(calculatedAmount)
          return calculatedAmount
        }

        // Don't show error for calculation, just set to 0
        setAmount(0)
        return 0
      } catch (_error) {
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
      } catch (_error) {
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
