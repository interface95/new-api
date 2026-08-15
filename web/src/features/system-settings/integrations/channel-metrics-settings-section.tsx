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
import { useEffect, useMemo, useRef } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'
import { safeNumberFieldProps } from '../utils/numeric-field'

const channelMetricsSchema = z.object({
  channel_metrics_setting: z.object({
    enabled: z.boolean(),
    flush_interval: z.coerce.number().min(1),
    bucket_time: z.enum(['minute', '5min', 'hour']),
    retention_days: z.coerce.number().min(0),
  }),
})

type ChannelMetricsFormInput = z.input<typeof channelMetricsSchema>
type ChannelMetricsFormValues = z.output<typeof channelMetricsSchema>

type FlatChannelMetricsDefaults = {
  'channel_metrics_setting.enabled': boolean
  'channel_metrics_setting.flush_interval': number
  'channel_metrics_setting.bucket_time': 'minute' | '5min' | 'hour'
  'channel_metrics_setting.retention_days': number
}

type ChannelMetricsSettingsSectionProps = {
  defaultValues: FlatChannelMetricsDefaults
}

const buildFormDefaults = (
  defaults: ChannelMetricsSettingsSectionProps['defaultValues']
): ChannelMetricsFormInput => ({
  channel_metrics_setting: {
    enabled: defaults['channel_metrics_setting.enabled'],
    flush_interval: defaults['channel_metrics_setting.flush_interval'],
    bucket_time: defaults['channel_metrics_setting.bucket_time'],
    retention_days: defaults['channel_metrics_setting.retention_days'],
  },
})

const normalizeDefaults = (
  defaults: ChannelMetricsSettingsSectionProps['defaultValues']
): FlatChannelMetricsDefaults => ({
  'channel_metrics_setting.enabled':
    defaults['channel_metrics_setting.enabled'],
  'channel_metrics_setting.flush_interval':
    defaults['channel_metrics_setting.flush_interval'],
  'channel_metrics_setting.bucket_time':
    defaults['channel_metrics_setting.bucket_time'],
  'channel_metrics_setting.retention_days':
    defaults['channel_metrics_setting.retention_days'],
})

const normalizeFormValues = (
  values: ChannelMetricsFormValues
): FlatChannelMetricsDefaults => ({
  'channel_metrics_setting.enabled': values.channel_metrics_setting.enabled,
  'channel_metrics_setting.flush_interval':
    values.channel_metrics_setting.flush_interval,
  'channel_metrics_setting.bucket_time':
    values.channel_metrics_setting.bucket_time,
  'channel_metrics_setting.retention_days':
    values.channel_metrics_setting.retention_days,
})

export function ChannelMetricsSettingsSection({
  defaultValues,
}: ChannelMetricsSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const baselineRef = useRef<FlatChannelMetricsDefaults>(
    normalizeDefaults(defaultValues)
  )
  const baselineSerializedRef = useRef<string>(
    JSON.stringify(normalizeDefaults(defaultValues))
  )

  const formDefaults = useMemo(
    () => buildFormDefaults(defaultValues),
    [defaultValues]
  )

  const form = useForm<
    ChannelMetricsFormInput,
    unknown,
    ChannelMetricsFormValues
  >({
    resolver: zodResolver(channelMetricsSchema),
    defaultValues: formDefaults,
  })

  useResetForm(form, formDefaults)

  useEffect(() => {
    const normalized = normalizeDefaults(defaultValues)
    const serialized = JSON.stringify(normalized)
    if (serialized === baselineSerializedRef.current) return
    baselineRef.current = normalized
    baselineSerializedRef.current = serialized
  }, [defaultValues])

  const channelMetricsEnabled = form.watch('channel_metrics_setting.enabled')

  const onSubmit = async (values: ChannelMetricsFormValues) => {
    const normalized = normalizeFormValues(values)
    const updates = (
      Object.keys(normalized) as Array<keyof FlatChannelMetricsDefaults>
    ).filter((key) => normalized[key] !== baselineRef.current[key])

    if (updates.length === 0) {
      toast.info(t('No changes to save'))
      return
    }

    for (const key of updates) {
      await updateOption.mutateAsync({
        key,
        value: normalized[key],
      })
    }

    baselineRef.current = normalized
    baselineSerializedRef.current = JSON.stringify(normalized)
  }

  return (
    <SettingsSection title={t('Channel performance metrics')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
          />

          <p className='text-muted-foreground text-xs'>
            {t('Collect per-attempt channel-side success-rate metrics.')}
          </p>

          <div className='grid grid-cols-1 gap-4 md:grid-cols-4'>
            <FormField
              control={form.control}
              name='channel_metrics_setting.enabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>
                      {t('Enable channel performance metrics')}
                    </FormLabel>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />
            <FormField
              control={form.control}
              name='channel_metrics_setting.flush_interval'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Flush interval (minutes)')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={1}
                      step={1}
                      {...safeNumberFieldProps(field)}
                      disabled={!channelMetricsEnabled}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='channel_metrics_setting.bucket_time'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Aggregation bucket')}</FormLabel>
                  <Select
                    items={[
                      { value: 'minute', label: t('1 minute') },
                      { value: '5min', label: t('5 minutes') },
                      { value: 'hour', label: t('1 hour') },
                    ]}
                    value={field.value}
                    onValueChange={field.onChange}
                    disabled={!channelMetricsEnabled}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent alignItemWithTrigger={false}>
                      <SelectGroup>
                        <SelectItem value='minute'>{t('1 minute')}</SelectItem>
                        <SelectItem value='5min'>{t('5 minutes')}</SelectItem>
                        <SelectItem value='hour'>{t('1 hour')}</SelectItem>
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='channel_metrics_setting.retention_days'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Retention days')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={0}
                      step={1}
                      {...safeNumberFieldProps(field)}
                      disabled={!channelMetricsEnabled}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('0 means data is kept permanently')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
