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
import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { updateSystemOption } from '../api'
import { SettingsSection } from '../components/settings-section'
import {
  buildBepusdtSettingsUpdates,
  saveBepusdtSettingsBatch,
  type BepusdtSettingsUpdate,
} from './bepusdt-settings-utils'

export interface BepusdtSettingsValues {
  BepusdtEnabled: boolean
  BepusdtGatewayURL: string
  BepusdtAuthToken: string
  BepusdtCurrencies: string
  BepusdtFiat: string
  BepusdtReturnURL: string
  BepusdtUnitPrice: number
  BepusdtMinTopUp: number
}

interface Props {
  defaultValues: BepusdtSettingsValues
}

export function BepusdtSettingsSection(props: Props) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [loading, setLoading] = useState(false)
  const form = useForm<BepusdtSettingsValues>({
    defaultValues: props.defaultValues,
  })

  useEffect(() => {
    form.reset(props.defaultValues)
  }, [props.defaultValues, form])

  const handleSave = async () => {
    const values = form.getValues()
    const enabled = !!values.BepusdtEnabled

    if (enabled && !values.BepusdtGatewayURL.trim()) {
      toast.error(t('BEpusdt gateway URL is required'))
      return
    }

    if (enabled && Number(values.BepusdtUnitPrice) <= 0) {
      toast.error(t('Unit price must be greater than 0'))
      return
    }

    if (enabled && Number(values.BepusdtMinTopUp) < 1) {
      toast.error(t('Minimum top-up amount must be at least 1'))
      return
    }

    setLoading(true)
    try {
      await saveBepusdtSettingsBatch(
        buildBepusdtSettingsUpdates(values),
        async (option: BepusdtSettingsUpdate) => {
          const data = await updateSystemOption(option)
          if (!data.success) {
            throw new Error(data.message || t('Update failed'))
          }
        },
        async () => {
          await queryClient.invalidateQueries({ queryKey: ['system-options'] })
        }
      )
      toast.success(t('Updated successfully'))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Update failed'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <SettingsSection title={t('BEpusdt Payment Gateway')}>
      <p className='text-muted-foreground text-sm'>
        {t('Configure BEpusdt USDT checkout integration for top-ups')}
      </p>

      <Alert>
        <AlertDescription className='text-xs'>
          {t(
            'BEpusdt callback URL: <ServerAddress>/api/bepusdt/webhook. Keep the auth token secret and configure the same token in BEpusdt.'
          )}
        </AlertDescription>
      </Alert>

      <div className='grid grid-cols-3 gap-4'>
        <div className='flex items-center gap-2'>
          <Switch
            checked={form.watch('BepusdtEnabled')}
            onCheckedChange={(value) => form.setValue('BepusdtEnabled', value)}
          />
          <Label>{t('Enable BEpusdt')}</Label>
        </div>
        <div className='grid gap-1.5'>
          <Label>{t('Fiat currency')}</Label>
          <Input placeholder='CNY' {...form.register('BepusdtFiat')} />
        </div>
      </div>

      <div className='grid gap-1.5'>
        <Label>{t('Allowed currencies')}</Label>
        <Input
          placeholder='USDT,USDC'
          {...form.register('BepusdtCurrencies')}
        />
      </div>

      <div className='grid grid-cols-2 gap-4'>
        <div className='grid gap-1.5'>
          <Label>{t('Gateway URL')}</Label>
          <Input
            placeholder='https://bepusdt.example.com'
            {...form.register('BepusdtGatewayURL')}
          />
        </div>
        <div className='grid gap-1.5'>
          <Label>{t('Auth token')}</Label>
          <Input
            type='password'
            placeholder={t('Leave blank to keep the existing key')}
            {...form.register('BepusdtAuthToken')}
          />
          <p className='text-muted-foreground text-xs'>
            {t('Stored value is not echoed back for security')}
          </p>
        </div>
      </div>

      <div className='grid grid-cols-3 gap-4'>
        <div className='grid gap-1.5'>
          <Label>{t('Payment return URL')}</Label>
          <Input
            placeholder='https://example.com/console/topup'
            {...form.register('BepusdtReturnURL')}
          />
          <p className='text-muted-foreground text-xs'>
            {t('Defaults to the wallet page when empty')}
          </p>
        </div>
        <div className='grid gap-1.5'>
          <Label>{t('Unit price (fiat / USD)')}</Label>
          <Input
            type='number'
            step={0.01}
            min={0}
            {...form.register('BepusdtUnitPrice', { valueAsNumber: true })}
          />
        </div>
        <div className='grid gap-1.5'>
          <Label>{t('Minimum top-up (USD)')}</Label>
          <Input
            type='number'
            min={1}
            {...form.register('BepusdtMinTopUp', { valueAsNumber: true })}
          />
        </div>
      </div>

      <Button onClick={handleSave} disabled={loading}>
        {loading ? t('Saving...') : t('Save BEpusdt settings')}
      </Button>
    </SettingsSection>
  )
}
