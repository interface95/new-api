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
import { useTranslation } from 'react-i18next'
import { useStatus } from '@/hooks/use-status'

const IFRAME_PROBE_TIMEOUT_MS = 4000

export function TopUpEmbed() {
  const { t } = useTranslation()
  const { status } = useStatus()
  const topUpLink = (status?.topup_link as string | undefined)?.trim() || ''
  const [loaded, setLoaded] = useState(false)
  const [probeFailed, setProbeFailed] = useState(false)

  useEffect(() => {
    setLoaded(false)
    setProbeFailed(false)
    if (!topUpLink) return
    const timer = window.setTimeout(() => {
      // Most browsers block onload from firing for cross-origin frames that
      // refused the embed (X-Frame-Options / CSP frame-ancestors). If the
      // load event never fires within a few seconds, surface the fallback.
      setProbeFailed(true)
    }, IFRAME_PROBE_TIMEOUT_MS)
    return () => window.clearTimeout(timer)
  }, [topUpLink])

  if (!topUpLink) {
    return (
      <div className='flex h-[calc(100vh-4rem)] items-center justify-center'>
        <p className='text-muted-foreground text-sm'>
          {t('The administrator has not configured a top-up link yet.')}
        </p>
      </div>
    )
  }

  return (
    <div className='relative flex h-[calc(100vh-4rem)] flex-col'>
      {probeFailed && !loaded && (
        <div className='border-amber-200 bg-amber-50 text-amber-900 dark:border-amber-700/40 dark:bg-amber-900/20 dark:text-amber-200 m-3 flex items-start gap-3 rounded-lg border px-4 py-3 text-sm'>
          <span aria-hidden='true' className='text-base leading-5'>
            ⚠️
          </span>
          <div className='flex-1'>
            <p>
              {t(
                'The top-up page could not be embedded (the target site may block iframes).'
              )}
            </p>
            <a
              href={topUpLink}
              target='_blank'
              rel='noopener noreferrer'
              className='mt-1 inline-block font-medium text-blue-600 underline hover:text-blue-800'
            >
              {t('Open in a new tab')} →
            </a>
          </div>
        </div>
      )}
      <iframe
        src={topUpLink}
        title={t('Top-Up')}
        className='h-full w-full flex-1 border-0'
        referrerPolicy='no-referrer-when-downgrade'
        sandbox='allow-forms allow-popups allow-popups-to-escape-sandbox allow-scripts allow-top-navigation-by-user-activation'
        onLoad={() => {
          setLoaded(true)
          setProbeFailed(false)
        }}
      />
    </div>
  )
}
