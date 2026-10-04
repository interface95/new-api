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
  Search,
  ChevronLeft,
  ChevronRight,
  UserRound,
  ReceiptText,
} from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { CopyButton } from '@/components/copy-button'
import { Dialog } from '@/components/dialog'
import { StatusBadge } from '@/components/status-badge'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { formatCurrencyFromUSD } from '@/lib/currency'
import { formatNumber, formatQuota } from '@/lib/format'

import { useBillingHistory } from '../../hooks/use-billing-history'
import {
  getStatusConfig,
  getPaymentMethodName,
  formatTimestamp,
} from '../../lib/billing'

interface BillingHistoryDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function BillingHistoryDialog({
  open,
  onOpenChange,
}: BillingHistoryDialogProps) {
  const { t } = useTranslation()
  const {
    records,
    total,
    page,
    pageSize,
    keyword,
    loading,
    completing,
    isAdmin,
    handlePageChange,
    handlePageSizeChange,
    handleSearch,
    handleCompleteOrder,
  } = useBillingHistory()

  const [confirmTradeNo, setConfirmTradeNo] = useState<string | null>(null)

  const totalPages = Math.ceil(total / pageSize)

  const handleConfirmComplete = async () => {
    if (confirmTradeNo) {
      const success = await handleCompleteOrder(confirmTradeNo)
      if (success) {
        setConfirmTradeNo(null)
      }
    }
  }

  return (
    <>
      <Dialog
        open={open}
        onOpenChange={onOpenChange}
        title={t('Billing History')}
        description={t(
          'View your topup transaction records and payment history'
        )}
        contentClassName='flex max-h-(--dialog-available-height) flex-col max-sm:w-screen max-sm:max-w-none max-sm:rounded-none max-sm:p-4 sm:max-w-4xl'
        contentHeight='auto'
        bodyClassName='space-y-3'
      >
        <div className='min-h-0 space-y-3'>
          {/* Search and Filter Bar */}
          <div className='flex items-center gap-2'>
            <div className='relative flex-1'>
              <Search className='text-muted-foreground absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2' />
              <Input
                placeholder={t('Search by order number...')}
                value={keyword}
                onChange={(e) => handleSearch(e.target.value)}
                className='h-9 pl-10'
              />
            </div>
            <Select
              items={[
                { value: '10', label: t('10 / page') },
                { value: '20', label: t('20 / page') },
                { value: '50', label: t('50 / page') },
                { value: '100', label: t('100 / page') },
              ]}
              value={pageSize.toString()}
              onValueChange={(value) =>
                value !== null && handlePageSizeChange(Number.parseInt(value))
              }
            >
              <SelectTrigger className='h-9 w-[92px] sm:w-32'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  <SelectItem value='10'>{t('10 / page')}</SelectItem>
                  <SelectItem value='20'>{t('20 / page')}</SelectItem>
                  <SelectItem value='50'>{t('50 / page')}</SelectItem>
                  <SelectItem value='100'>{t('100 / page')}</SelectItem>
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>

          {/* Records List */}
          <div className='max-h-[min(54vh,520px)] overflow-y-auto pr-1'>
            {loading && (
              <div className='space-y-3'>
                {['first', 'second', 'third', 'fourth', 'fifth'].map(
                  (placeholder) => (
                    <div
                      key={placeholder}
                      className='rounded-lg border p-3 sm:p-4'
                    >
                      <div className='flex items-start justify-between'>
                        <div className='flex-1 space-y-2'>
                          <Skeleton className='h-4 w-48' />
                          <Skeleton className='h-3 w-32' />
                        </div>
                        <Skeleton className='h-5 w-16' />
                      </div>
                      <div className='mt-3 grid grid-cols-2 gap-3 sm:grid-cols-3 sm:gap-4'>
                        <Skeleton className='h-3 w-full' />
                        <Skeleton className='h-3 w-full' />
                        <Skeleton className='h-3 w-full' />
                      </div>
                    </div>
                  )
                )}
              </div>
            )}
            {!loading && records.length === 0 && (
              <div className='text-muted-foreground flex min-h-40 flex-col items-center justify-center py-10 text-center'>
                <p className='text-sm font-medium'>
                  {t('No billing records found')}
                </p>
                <p className='mt-1 text-xs'>
                  {keyword
                    ? t('Try adjusting your search')
                    : t('Your transaction history will appear here')}
                </p>
              </div>
            )}
            {!loading && records.length > 0 && (
              <div className='space-y-3'>
                {records.map((record) => {
                  const statusConfig = getStatusConfig(record.status)
                  return (
                    <article
                      key={record.id}
                      className='bg-card overflow-hidden rounded-xl border'
                    >
                      <header className='flex items-start justify-between gap-3 px-4 pt-4 sm:px-5'>
                        <div className='flex min-w-0 flex-1 items-start gap-3'>
                          <Avatar size='lg' aria-hidden='true'>
                            <AvatarFallback>
                              {isAdmin ? (
                                <UserRound className='size-5' />
                              ) : (
                                <ReceiptText className='size-5' />
                              )}
                            </AvatarFallback>
                          </Avatar>
                          <div className='flex min-w-0 flex-1 flex-col gap-1'>
                            {isAdmin ? (
                              <dl>
                                <dt className='sr-only'>{t('Username')}</dt>
                                <dd className='text-base leading-snug font-semibold break-all'>
                                  {record.user?.username ||
                                    t('User unavailable')}
                                </dd>
                              </dl>
                            ) : (
                              <p className='text-base leading-snug font-semibold'>
                                {t('Recharge')}
                              </p>
                            )}
                            {isAdmin && record.user_id != null && (
                              <StatusBadge
                                label={`${t('User ID')}: ${record.user_id}`}
                                variant='neutral'
                                type='text'
                                className='w-fit text-xs'
                                copyText={String(record.user_id)}
                              />
                            )}
                          </div>
                        </div>
                        <StatusBadge
                          label={t(statusConfig.label)}
                          variant={statusConfig.variant}
                          type='badge'
                          className='shrink-0'
                          showDot
                          copyable={false}
                        />
                      </header>

                      <dl className='grid grid-cols-2 items-start gap-x-5 gap-y-4 px-4 py-4 sm:grid-cols-3 sm:px-5 sm:py-5'>
                        <div className='col-span-2 flex min-w-0 flex-col gap-1.5 sm:col-span-1'>
                          <dt className='text-muted-foreground text-xs'>
                            {t('Recharge Amount')}
                          </dt>
                          <dd className='text-2xl leading-tight font-semibold tracking-tight break-all tabular-nums sm:text-3xl'>
                            {formatCurrencyFromUSD(record.amount, {
                              digitsLarge: 2,
                              digitsSmall: 2,
                              abbreviate: false,
                            })}
                          </dd>
                          <dd className='text-muted-foreground flex flex-wrap items-center gap-x-2 gap-y-1 text-xs'>
                            <span>
                              {t('Payment')}{' '}
                              <span className='text-foreground font-medium tabular-nums'>
                                {formatNumber(record.money)}
                              </span>
                            </span>
                            <Badge variant='secondary'>
                              <span className='sr-only'>
                                {t('Payment Method')}:{' '}
                              </span>
                              {getPaymentMethodName(record.payment_method, t)}
                            </Badge>
                          </dd>
                        </div>
                        {isAdmin && (
                          <>
                            <div className='flex min-w-0 flex-col gap-1.5'>
                              <dt className='text-muted-foreground text-xs'>
                                {t('Current Quota')}
                              </dt>
                              <dd className='text-lg leading-tight font-semibold break-all tabular-nums sm:text-xl'>
                                {record.user
                                  ? formatQuota(record.user.quota)
                                  : '—'}
                              </dd>
                            </div>
                            <div className='flex min-w-0 flex-col gap-1.5'>
                              <dt className='text-muted-foreground text-xs'>
                                {t('Total Historical Quota')}
                              </dt>
                              <dd className='text-lg leading-tight font-semibold break-all tabular-nums sm:text-xl'>
                                {record.user
                                  ? formatQuota(
                                      record.user.quota + record.user.used_quota
                                    )
                                  : '—'}
                              </dd>
                            </div>
                          </>
                        )}
                      </dl>

                      <Separator />
                      <footer className='bg-muted/25 flex flex-col gap-3 px-4 py-3 sm:flex-row sm:items-center sm:justify-between sm:px-5'>
                        <div className='flex min-w-0 flex-1 flex-col gap-1'>
                          <div className='text-muted-foreground flex min-w-0 items-center gap-1.5'>
                            <code className='min-w-0 font-mono text-xs break-all'>
                              {record.trade_no}
                            </code>
                            <CopyButton
                              value={record.trade_no}
                              aria-label={`${t('Copy')} ${record.trade_no}`}
                              className='size-11 sm:size-6'
                              iconClassName='size-3.5'
                            />
                          </div>
                          <time className='text-muted-foreground text-xs tabular-nums'>
                            {formatTimestamp(record.create_time)}
                          </time>
                        </div>
                        {isAdmin && record.status === 'pending' && (
                          <Button
                            size='sm'
                            variant='outline'
                            className='h-11 shrink-0 px-4 sm:h-8'
                            onClick={() => setConfirmTradeNo(record.trade_no)}
                            disabled={completing}
                          >
                            {t('Complete Order')}
                          </Button>
                        )}
                      </footer>
                    </article>
                  )
                })}
              </div>
            )}
          </div>

          {/* Pagination */}
          {!loading && records.length > 0 && (
            <div className='flex flex-col items-center gap-3 border-t pt-4 sm:flex-row sm:items-center sm:justify-between'>
              <div className='text-muted-foreground text-xs sm:text-sm'>
                {t('Showing')} {(page - 1) * pageSize + 1}-
                {Math.min(page * pageSize, total)} {t('of')} {total}
              </div>
              <div className='flex items-center gap-2'>
                <Button
                  variant='outline'
                  size='sm'
                  onClick={() => handlePageChange(page - 1)}
                  disabled={page <= 1}
                  className='h-8 w-8 p-0'
                >
                  <ChevronLeft className='h-4 w-4' />
                </Button>
                <div className='text-muted-foreground flex items-center gap-1 text-sm'>
                  <span className='font-medium'>{page}</span>
                  <span>/</span>
                  <span>{totalPages}</span>
                </div>
                <Button
                  variant='outline'
                  size='sm'
                  onClick={() => handlePageChange(page + 1)}
                  disabled={page >= totalPages}
                  className='h-8 w-8 p-0'
                >
                  <ChevronRight className='h-4 w-4' />
                </Button>
              </div>
            </div>
          )}
        </div>
      </Dialog>

      {/* Confirm Complete Order Dialog */}
      <ConfirmDialog
        open={!!confirmTradeNo}
        onOpenChange={(open) => !open && setConfirmTradeNo(null)}
        title={t('Complete Order')}
        desc={t(
          'Are you sure you want to manually complete this order? The user will be credited with the corresponding quota.'
        )}
        confirmText={completing ? t('Processing...') : t('Confirm')}
        handleConfirm={handleConfirmComplete}
        isLoading={completing}
      />
    </>
  )
}
