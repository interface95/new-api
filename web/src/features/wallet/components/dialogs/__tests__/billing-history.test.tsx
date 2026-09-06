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
import {
  act,
  cleanup,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Toaster, toast } from 'sonner'
import {
  afterAll,
  afterEach,
  beforeAll,
  beforeEach,
  describe,
  expect,
  test,
  vi,
} from 'vitest'

import { api } from '@/lib/api'
import { formatQuota } from '@/lib/format'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import type { TopupRecord } from '../../../types'
import { BillingHistoryDialog } from '../billing-history-dialog'

type BillingTransport = {
  get: (url: string) => Promise<{ data: unknown }>
  post: (url: string, body: unknown) => Promise<{ data: unknown }>
}

const apiClient = api as unknown as BillingTransport
const originalGetAnimations = Object.getOwnPropertyDescriptor(
  HTMLElement.prototype,
  'getAnimations'
)
const originalConfig = useSystemConfigStore.getState().config
const pendingRecord = {
  id: 41,
  user_id: 7,
  amount: 20,
  money: 140,
  trade_no: 'TOPUP-41',
  payment_method: 'bepusdt',
  create_time: 1_700_000_000,
  status: 'pending' as const,
  user: {
    username: 'wallet-owner',
    quota: 4_500_000,
    used_quota: 1_500_000,
  },
}

function billingPage(items: TopupRecord[], total = items.length) {
  return { data: { success: true, data: { items, total } } }
}

function accountField(label: string): HTMLElement {
  const term = screen.getByText(label, { selector: 'dt' })
  const definition = term.nextElementSibling
  if (!(definition instanceof HTMLElement)) {
    throw new Error(`Missing account value for ${label}`)
  }
  return definition
}

function renderHistory() {
  return render(
    <>
      <BillingHistoryDialog open onOpenChange={() => undefined} />
      <Toaster duration={60_000} />
    </>
  )
}

beforeAll(() => {
  Object.defineProperty(HTMLElement.prototype, 'getAnimations', {
    configurable: true,
    value: () => [],
  })
})

beforeEach(() => {
  useAuthStore.getState().auth.setUser({
    id: 1,
    username: 'billing-admin',
    role: ROLE.ADMIN,
  })
  useSystemConfigStore.getState().setConfig({
    currency: { ...DEFAULT_CURRENCY_CONFIG },
  })
})

afterEach(() => {
  cleanup()
  toast.dismiss()
  vi.restoreAllMocks()
  useAuthStore.getState().auth.reset()
  useSystemConfigStore.getState().setConfig(originalConfig)
  localStorage.clear()
})

afterAll(() => {
  if (originalGetAnimations) {
    Object.defineProperty(
      HTMLElement.prototype,
      'getAnimations',
      originalGetAnimations
    )
  } else {
    Reflect.deleteProperty(HTMLElement.prototype, 'getAnimations')
  }
})

describe('Billing history user information', () => {
  test.each([
    {
      case: 'positive balances',
      quota: 4_500_000,
      used: 1_500_000,
      total: 6_000_000,
    },
    { case: 'zero balances', quota: 0, used: 0, total: 0 },
    {
      case: 'a negative current balance',
      quota: -500_000,
      used: 1_500_000,
      total: 1_000_000,
    },
    {
      case: 'balances above the 32-bit range',
      quota: 4_000_000_000_000_000,
      used: 4_000_000_000_000_000,
      total: 8_000_000_000_000_000,
    },
  ])(
    'shows the owner and correctly formatted administrator quotas for $case',
    async ({ quota, used, total }) => {
      const record = {
        ...pendingRecord,
        user: { ...pendingRecord.user, quota, used_quota: used },
      }
      const get = vi
        .spyOn(apiClient, 'get')
        .mockResolvedValue(billingPage([record]))

      renderHistory()

      await screen.findByText(pendingRecord.trade_no)
      expect(get).toHaveBeenCalledWith('/api/user/topup?p=1&page_size=10')
      expect(accountField('Username')).toHaveTextContent('wallet-owner')
      expect(accountField('Current Quota').textContent).toBe(formatQuota(quota))
      expect(accountField('Total Historical Quota').textContent).toBe(
        formatQuota(total)
      )
      expect(
        screen.getByRole('button', { name: 'Complete Order' })
      ).toBeEnabled()
    }
  )

  test.each([undefined, null])(
    'shows unavailable account details instead of fabricated zero quotas when user is %s',
    async (user) => {
      vi.spyOn(apiClient, 'get').mockResolvedValue(
        billingPage([{ ...pendingRecord, user }])
      )

      renderHistory()

      await screen.findByText(pendingRecord.trade_no)
      expect(accountField('Username')).toHaveTextContent('User unavailable')
      expect(accountField('Current Quota').textContent).toBe('—')
      expect(accountField('Total Historical Quota').textContent).toBe('—')
    }
  )

  test('requests only self history and hides administrator account details for an ordinary user', async () => {
    useAuthStore.getState().auth.setUser({
      id: 7,
      username: 'wallet-owner',
      role: ROLE.USER,
    })
    const get = vi
      .spyOn(apiClient, 'get')
      .mockResolvedValue(billingPage([pendingRecord]))

    renderHistory()

    await screen.findByText(pendingRecord.trade_no)
    expect(get).toHaveBeenCalledWith('/api/user/topup/self?p=1&page_size=10')
    expect(screen.queryByText('Username')).not.toBeInTheDocument()
    expect(screen.queryByText('wallet-owner')).not.toBeInTheDocument()
    expect(screen.queryByText('Current Quota')).not.toBeInTheDocument()
    expect(screen.queryByText('Total Historical Quota')).not.toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: 'Complete Order' })
    ).not.toBeInTheDocument()
  })

  test('requires confirmation before completing an order and refreshes both its status and account balance', async () => {
    const user = userEvent.setup()
    const completedRecord: TopupRecord = {
      ...pendingRecord,
      status: 'success',
      user: { ...pendingRecord.user, quota: 9_500_000 },
    }
    const get = vi
      .spyOn(apiClient, 'get')
      .mockResolvedValueOnce(billingPage([pendingRecord]))
      .mockResolvedValue(billingPage([completedRecord]))
    let finishCompletion!: (response: { data: unknown }) => void
    const completion = new Promise<{ data: unknown }>((resolve) => {
      finishCompletion = resolve
    })
    const post = vi.spyOn(apiClient, 'post').mockReturnValue(completion)

    renderHistory()

    await screen.findByText(pendingRecord.trade_no)
    await user.click(screen.getByRole('button', { name: 'Complete Order' }))
    const confirmation = await screen.findByRole('alertdialog', {
      name: 'Complete Order',
    })
    expect(post).not.toHaveBeenCalled()
    await user.click(
      within(confirmation).getByRole('button', { name: 'Confirm' })
    )
    expect(post).toHaveBeenCalledWith('/api/user/topup/complete', {
      trade_no: 'TOPUP-41',
    })
    expect(
      within(confirmation).getByRole('button', { name: 'Processing...' })
    ).toBeDisabled()

    await act(async () => {
      finishCompletion({ data: { success: true } })
    })

    await waitFor(() =>
      expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    )
    expect(get).toHaveBeenCalledTimes(2)
    expect(screen.getByText('Success')).toBeVisible()
    expect(screen.queryByText('Pending')).not.toBeInTheDocument()
    expect(accountField('Username')).toHaveTextContent('wallet-owner')
    expect(accountField('Current Quota').textContent).toBe(
      formatQuota(9_500_000)
    )
    expect(accountField('Total Historical Quota').textContent).toBe(
      formatQuota(11_000_000)
    )
    expect(
      screen.queryByRole('button', { name: 'Complete Order' })
    ).not.toBeInTheDocument()
  })

  test('retains the pending order and account balances when completion is rejected and shows the error', async () => {
    const user = userEvent.setup()
    const get = vi
      .spyOn(apiClient, 'get')
      .mockResolvedValue(billingPage([pendingRecord]))
    vi.spyOn(apiClient, 'post').mockResolvedValue({
      data: { success: false, message: 'Payment provider rejected completion' },
    })

    renderHistory()

    await screen.findByText(pendingRecord.trade_no)
    await user.click(screen.getByRole('button', { name: 'Complete Order' }))
    const confirmation = await screen.findByRole('alertdialog', {
      name: 'Complete Order',
    })
    await user.click(
      within(confirmation).getByRole('button', { name: 'Confirm' })
    )

    expect(
      await screen.findByText('Payment provider rejected completion')
    ).toBeVisible()
    expect(
      within(confirmation).getByRole('button', { name: 'Confirm' })
    ).toBeEnabled()
    expect(get).toHaveBeenCalledTimes(1)
    expect(accountField('Username')).toHaveTextContent('wallet-owner')
    expect(accountField('Current Quota').textContent).toBe(
      formatQuota(4_500_000)
    )
    expect(accountField('Total Historical Quota').textContent).toBe(
      formatQuota(6_000_000)
    )
    expect(screen.getByText('Pending')).toBeInTheDocument()
    expect(screen.queryByText('Success')).not.toBeInTheDocument()
  })

  test('allows a long unbroken username to wrap and keeps order completion keyboard-accessible', async () => {
    const user = userEvent.setup()
    const username = 'account-with-an-unbroken-identifier-'.repeat(8)
    vi.spyOn(apiClient, 'get').mockResolvedValue(
      billingPage([
        { ...pendingRecord, user: { ...pendingRecord.user, username } },
      ])
    )
    const post = vi.spyOn(apiClient, 'post')

    renderHistory()

    const owner = await screen.findByText(username)
    expect(owner).toHaveClass('break-all')
    const completeButton = screen.getByRole('button', {
      name: 'Complete Order',
    })
    expect(completeButton).toBeVisible()
    expect(completeButton).toBeEnabled()
    completeButton.focus()
    expect(completeButton).toHaveFocus()
    await user.keyboard('{Enter}')

    expect(
      await screen.findByRole('alertdialog', { name: 'Complete Order' })
    ).toBeVisible()
    expect(post).not.toHaveBeenCalled()
  })

  test('refreshes the account details with the searched order instead of keeping the previous owner', async () => {
    const user = userEvent.setup()
    const matchedRecord: TopupRecord = {
      ...pendingRecord,
      id: 77,
      user_id: 9,
      trade_no: 'TOPUP-77',
      user: {
        username: 'searched-owner',
        quota: 2_000_000,
        used_quota: 3_000_000,
      },
    }
    const get = vi.spyOn(apiClient, 'get').mockImplementation(async (url) => {
      const requested = new URL(url, 'http://localhost')
      return requested.searchParams.get('keyword') === matchedRecord.trade_no
        ? billingPage([matchedRecord])
        : billingPage([pendingRecord])
    })

    renderHistory()

    await screen.findByText(pendingRecord.trade_no)
    await user.type(
      screen.getByPlaceholderText('Search by order number...'),
      matchedRecord.trade_no
    )

    await screen.findByText(matchedRecord.trade_no)
    expect(get).toHaveBeenLastCalledWith(
      '/api/user/topup?p=1&page_size=10&keyword=TOPUP-77'
    )
    expect(screen.queryByText(pendingRecord.trade_no)).not.toBeInTheDocument()
    expect(accountField('Username')).toHaveTextContent('searched-owner')
    expect(screen.queryByText('wallet-owner')).not.toBeInTheDocument()
    expect(accountField('Current Quota').textContent).toBe(
      formatQuota(2_000_000)
    )
    expect(accountField('Total Historical Quota').textContent).toBe(
      formatQuota(5_000_000)
    )
  })

  test('keeps each order and its account information in a separate article and completes only the selected order', async () => {
    const user = userEvent.setup()
    const anotherRecord: TopupRecord = {
      ...pendingRecord,
      id: 77,
      user_id: 9,
      trade_no: 'TOPUP-77',
      user: {
        username: 'another-owner',
        quota: 2_000_000,
        used_quota: 3_000_000,
      },
    }
    vi.spyOn(apiClient, 'get')
      .mockResolvedValueOnce(billingPage([pendingRecord, anotherRecord]))
      .mockResolvedValue(
        billingPage([pendingRecord, { ...anotherRecord, status: 'success' }])
      )
    const post = vi.spyOn(apiClient, 'post').mockResolvedValue({
      data: { success: true },
    })

    renderHistory()

    await screen.findByText(anotherRecord.trade_no)
    const articles = screen.getAllByRole('article')
    expect(articles).toHaveLength(2)
    const firstArticle = articles.find((article) =>
      within(article).queryByText(pendingRecord.trade_no)
    )
    const selectedArticle = articles.find((article) =>
      within(article).queryByText(anotherRecord.trade_no)
    )
    expect(firstArticle).toHaveTextContent('wallet-owner')
    expect(firstArticle).not.toHaveTextContent('another-owner')
    expect(selectedArticle).toHaveTextContent('another-owner')
    expect(selectedArticle).not.toHaveTextContent('wallet-owner')
    if (!selectedArticle) {
      throw new Error('The selected order must have its own article')
    }
    expect(
      within(selectedArticle).getByText(formatQuota(2_000_000))
    ).toBeVisible()
    expect(
      within(selectedArticle).getByText(formatQuota(5_000_000))
    ).toBeVisible()
    await user.click(
      within(selectedArticle).getByRole('button', { name: 'Complete Order' })
    )
    const confirmation = await screen.findByRole('alertdialog', {
      name: 'Complete Order',
    })
    await user.click(
      within(confirmation).getByRole('button', { name: 'Confirm' })
    )

    expect(post).toHaveBeenCalledWith('/api/user/topup/complete', {
      trade_no: 'TOPUP-77',
    })
    await waitFor(() =>
      expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    )
    expect(
      screen.getAllByRole('button', { name: 'Complete Order' })
    ).toHaveLength(1)
  })

  test('keeps a long order number fully readable and copies that exact number through an explicitly named button', async () => {
    const user = userEvent.setup()
    const writeText = vi
      .spyOn(navigator.clipboard, 'writeText')
      .mockResolvedValue()
    const tradeNo = 'BEPUSDT-20260906-A7C52D09003C44E194B3FA76F6CF9238-TOPUP-41'
    vi.spyOn(apiClient, 'get').mockResolvedValue(
      billingPage([{ ...pendingRecord, trade_no: tradeNo }])
    )

    renderHistory()

    const orderNumber = await screen.findByText(tradeNo)
    const copyButton = screen.getByRole('button', {
      name: (name) => /copy/i.test(name) && name.includes(tradeNo),
    })
    expect(orderNumber).toBeVisible()
    expect(orderNumber).not.toHaveClass('truncate')
    expect(orderNumber).toHaveClass('break-all')
    await user.click(copyButton)

    await waitFor(() => expect(writeText).toHaveBeenCalledWith(tradeNo))
  })
})
