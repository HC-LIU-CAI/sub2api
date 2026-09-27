/**
 * Admin device identity endpoints.
 *
 * Device identities are owned by the server-side custom identity service.
 * Keep this wrapper separate from the generic account API so the custom
 * contract remains easy to update without spreading endpoint details across
 * the view.
 */

import type { Account, AccountListItem } from '@/types'
import accountsAPI from './accounts'
import { apiClient } from '../client'

export type DeviceIdentityPlatform = 'openai' | 'anthropic'

export interface DeviceIdentityAccount extends AccountListItem {
  extra?: Record<string, unknown> & {
    openai_device_id?: string
    claude_user_id?: string
    enable_tls_fingerprint?: boolean
  }
  enable_tls_fingerprint?: boolean | null
}

export interface DeviceIdentityFilters {
  platform?: DeviceIdentityPlatform
  search?: string
}

export interface DeviceIdentityBatchResult {
  updated: number
  skipped: number
  failed: number
  results?: Array<{
    account_id?: number
    id?: number
    success?: boolean
    status?: string
    error?: string
    message?: string
  }>
}

export interface DeviceIdentityResetResult {
  updated: boolean
  account_id: number
}

const PAGE_SIZE = 200
const MAX_PAGES = 10

async function listPlatform(
  platform: DeviceIdentityPlatform,
  type: 'oauth' | 'setup-token',
  filters: DeviceIdentityFilters
): Promise<DeviceIdentityAccount[]> {
  const items: DeviceIdentityAccount[] = []
  for (let page = 1; page <= MAX_PAGES; page += 1) {
    const result = await accountsAPI.list(page, PAGE_SIZE, {
      platform,
      type,
      search: filters.search,
      lite: '0'
    })
    items.push(...(result.items as DeviceIdentityAccount[]))
    if (items.length >= result.total || result.items.length < PAGE_SIZE) break
  }
  return items
}

export async function list(filters: DeviceIdentityFilters = {}): Promise<DeviceIdentityAccount[]> {
  const platforms: DeviceIdentityPlatform[] = filters.platform ? [filters.platform] : ['openai', 'anthropic']
  const requests: Promise<DeviceIdentityAccount[]>[] = []
  for (const platform of platforms) {
    requests.push(listPlatform(platform, 'oauth', filters))
    if (platform === 'anthropic') requests.push(listPlatform(platform, 'setup-token', filters))
  }
  const result = (await Promise.all(requests)).flat()
  return result.sort((a, b) => a.id - b.id)
}

export async function getById(id: number): Promise<DeviceIdentityAccount> {
  return (await accountsAPI.getById(id)) as DeviceIdentityAccount
}

export async function ensure(force = false): Promise<DeviceIdentityBatchResult> {
  const { data } = await apiClient.post<DeviceIdentityBatchResult>('/admin/accounts/device-identity/ensure', { force })
  return data
}

export async function reset(id: number): Promise<DeviceIdentityResetResult> {
  const { data } = await apiClient.post<DeviceIdentityResetResult>(`/admin/accounts/${id}/device-identity/reset`)
  return data
}

export async function setTLSFingerprint(id: number, enabled: boolean): Promise<Account> {
  return accountsAPI.update(id, { extra: { enable_tls_fingerprint: enabled } })
}

export async function enableTLSFingerprint(id: number): Promise<Account> {
  return setTLSFingerprint(id, true)
}

export async function enableTLSFingerprintMany(
  accountIds: number[],
  options?: {
    concurrency?: number
    onProgress?: (progress: { done: number; total: number; success: number; failed: number }) => void
  }
): Promise<{ success: number; failed: number }> {
  const concurrency = Math.max(1, Math.min(options?.concurrency ?? 4, 8))
  let cursor = 0
  let success = 0
  let failed = 0

  const worker = async (): Promise<void> => {
    while (true) {
      const index = cursor
      cursor += 1
      if (index >= accountIds.length) return
      try {
        await setTLSFingerprint(accountIds[index], true)
        success += 1
      } catch {
        failed += 1
      }
      options?.onProgress?.({
        done: success + failed,
        total: accountIds.length,
        success,
        failed
      })
    }
  }

  await Promise.all(Array.from({ length: Math.min(concurrency, accountIds.length) }, () => worker()))
  return { success, failed }
}

export function getDeviceId(account: DeviceIdentityAccount): string {
  const key = account.platform === 'anthropic' ? 'claude_user_id' : 'openai_device_id'
  const value = account.extra?.[key]
  return typeof value === 'string' ? value : ''
}

export function isTLSFingerprintEnabled(account: DeviceIdentityAccount): boolean {
  return account.platform === 'anthropic' && (
    account.enable_tls_fingerprint === true || account.extra?.enable_tls_fingerprint === true
  )
}

export const deviceIdentityAPI = {
  list,
  getById,
  ensure,
  reset,
  enableTLSFingerprint,
  setTLSFingerprint,
  enableTLSFingerprintMany,
  getDeviceId,
  isTLSFingerprintEnabled
}

export default deviceIdentityAPI
