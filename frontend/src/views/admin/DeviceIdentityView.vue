<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <div class="relative w-full sm:w-72">
            <Icon name="search" size="md" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
            <input
              v-model="search"
              class="input pl-10"
              type="search"
              :placeholder="t('admin.deviceIdentity.searchPlaceholder')"
            />
          </div>
          <Select v-model="platformFilter" class="w-40" :options="platformOptions" />
          <Select v-model="statusFilter" class="w-36" :options="statusOptions" />
          <div class="flex flex-1 flex-wrap items-center justify-end gap-2">
            <button class="btn btn-secondary" :disabled="loading || busy" :title="t('common.refresh')" @click="loadAccounts">
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
            <button
              class="btn btn-secondary"
              :disabled="tlsTargetIds.length === 0 || busy"
              @click="confirmAction = 'tls'"
            >
              <Icon name="shield" size="md" class="mr-1.5" />
              {{ t('admin.deviceIdentity.enableTls') }}
            </button>
            <button class="btn btn-secondary" :disabled="busy" @click="confirmAction = 'missing'">
              <Icon name="sparkles" size="md" class="mr-1.5" />
              {{ t('admin.deviceIdentity.generateMissing') }}
            </button>
            <button class="btn btn-danger" :disabled="busy || rows.length === 0" @click="confirmAction = 'all'">
              <Icon name="refresh" size="md" class="mr-1.5" />
              {{ t('admin.deviceIdentity.resetAll') }}
            </button>
          </div>
        </div>
        <div class="mt-3 flex flex-wrap items-center gap-3 text-sm text-gray-500 dark:text-gray-400">
          <span>{{ t('admin.deviceIdentity.selected', { count: selectedIds.size }) }}</span>
          <span>{{ t('admin.deviceIdentity.stats.total', { count: rows.length }) }}</span>
          <span class="text-emerald-600 dark:text-emerald-400">{{ t('admin.deviceIdentity.stats.configured', { count: configuredCount }) }}</span>
          <span class="text-amber-600 dark:text-amber-400">{{ t('admin.deviceIdentity.stats.missing', { count: missingCount }) }}</span>
          <span v-if="progress.total" class="ml-auto">
            {{ t('admin.deviceIdentity.progress', progress) }}
          </span>
        </div>
      </template>

      <template #table>
        <div class="table-wrapper overflow-x-auto">
          <table class="w-full min-w-[980px]">
            <thead>
              <tr>
                <th class="w-12 text-center">
                  <input
                    type="checkbox"
                    class="h-4 w-4 rounded border-gray-300 text-primary-600"
                    :checked="allPageSelected"
                    :indeterminate="somePageSelected"
                    :aria-label="t('admin.deviceIdentity.selectAll')"
                    @change="togglePageSelection"
                  />
                </th>
                <th>{{ t('admin.deviceIdentity.columns.account') }}</th>
                <th>{{ t('admin.deviceIdentity.columns.platform') }}</th>
                <th>{{ t('admin.deviceIdentity.columns.deviceId') }}</th>
                <th>{{ t('admin.deviceIdentity.columns.tls') }}</th>
                <th>{{ t('admin.deviceIdentity.columns.status') }}</th>
                <th class="text-right">{{ t('admin.deviceIdentity.columns.actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="loading">
                <td colspan="7" class="py-12 text-center text-gray-500">{{ t('common.loading') }}</td>
              </tr>
              <tr v-else-if="pageRows.length === 0">
                <td colspan="7" class="py-12 text-center text-gray-500">
                  <Icon name="inbox" size="xl" class="mx-auto mb-3 text-gray-400" />
                  {{ t('admin.deviceIdentity.empty') }}
                </td>
              </tr>
              <tr v-for="row in pageRows" v-else :key="row.id" class="hover:bg-gray-50 dark:hover:bg-dark-700/50">
                <td class="text-center">
                  <input
                    type="checkbox"
                    class="h-4 w-4 rounded border-gray-300 text-primary-600"
                    :checked="selectedIds.has(row.id)"
                    @change="toggleRow(row.id)"
                  />
                </td>
                <td>
                  <div class="max-w-[240px] truncate font-medium text-gray-900 dark:text-white">{{ row.name || `#${row.id}` }}</div>
                  <div class="mt-1 text-xs text-gray-500">#{{ row.id }} · {{ row.type }}</div>
                </td>
                <td><span class="badge badge-primary">{{ platformLabel(row.platform) }}</span></td>
                <td>
                  <div v-if="getDeviceId(row)" class="flex max-w-[360px] items-center gap-2">
                    <code class="truncate text-xs text-gray-600 dark:text-gray-300">{{ getDeviceId(row) }}</code>
                    <button class="text-gray-400 hover:text-primary-600" :title="t('admin.deviceIdentity.copy')" @click="copyDeviceId(getDeviceId(row))">
                      <Icon name="copy" size="sm" />
                    </button>
                  </div>
                  <span v-else class="text-amber-600 dark:text-amber-400">{{ t('admin.deviceIdentity.missing') }}</span>
                </td>
                <td>
                  <Toggle
                    v-if="row.platform === 'anthropic'"
                    :model-value="isTLSFingerprintEnabled(row)"
                    :disabled="busy"
                    @update:model-value="setTLSFingerprint(row, $event)"
                  />
                  <span v-else class="text-gray-400">-</span>
                </td>
                <td>
                  <span :class="['badge', row.status === 'active' ? 'badge-success' : 'badge-gray']">{{ row.status }}</span>
                </td>
                <td>
                  <div class="flex justify-end">
                    <button class="btn btn-ghost btn-sm" :disabled="busy" :title="t('admin.deviceIdentity.resetOne')" @click="resetOne(row)">
                      <Icon name="refresh" size="sm" />
                      <span class="ml-1">{{ t('admin.deviceIdentity.resetOne') }}</span>
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </template>

      <template #pagination>
        <Pagination
          v-if="filteredRows.length > 0"
          :page="page"
          :total="filteredRows.length"
          :page-size="pageSize"
          @update:page="page = $event"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>

    <ConfirmDialog
      :show="confirmAction !== null"
      :title="confirmTitle"
      :message="confirmMessage"
      :confirm-text="t('common.confirm')"
      :cancel-text="t('common.cancel')"
      :danger="confirmAction === 'all'"
      @confirm="runConfirmedAction"
      @cancel="cancelConfirmation"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useDebounceFn } from '@vueuse/core'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import deviceIdentityAPI, {
  type DeviceIdentityAccount,
  type DeviceIdentityPlatform,
  getDeviceId,
  isTLSFingerprintEnabled
} from '@/api/admin/deviceIdentity'

const { t } = useI18n()
const appStore = useAppStore()
const rows = ref<DeviceIdentityAccount[]>([])
const loading = ref(false)
const busy = ref(false)
const search = ref('')
const platformFilter = ref<'all' | DeviceIdentityPlatform>('all')
const statusFilter = ref<'all' | 'missing' | 'ready'>('all')
const selectedIds = ref(new Set<number>())
const page = ref(1)
const pageSize = ref(20)
const confirmAction = ref<'missing' | 'all' | 'tls' | 'one' | null>(null)
const pendingResetRow = ref<DeviceIdentityAccount | null>(null)
const progress = ref({ done: 0, total: 0, success: 0, failed: 0 })

const platformOptions = computed<SelectOption[]>(() => [
  { value: 'all', label: t('admin.deviceIdentity.filters.allPlatforms') },
  { value: 'openai', label: 'OpenAI' },
  { value: 'anthropic', label: 'Claude' }
])
const statusOptions = computed<SelectOption[]>(() => [
  { value: 'all', label: t('admin.deviceIdentity.filters.allStatus') },
  { value: 'missing', label: t('admin.deviceIdentity.filters.missing') },
  { value: 'ready', label: t('admin.deviceIdentity.filters.ready') }
])

const filteredRows = computed(() => {
  const query = search.value.trim().toLowerCase()
  return rows.value.filter((row) => {
    const matchesPlatform = platformFilter.value === 'all' || row.platform === platformFilter.value
    const hasDeviceId = Boolean(getDeviceId(row))
    const matchesStatus = statusFilter.value === 'all' ||
      (statusFilter.value === 'ready' && hasDeviceId) ||
      (statusFilter.value === 'missing' && !hasDeviceId)
    const matchesSearch = !query || row.name.toLowerCase().includes(query) || String(row.id).includes(query)
    return matchesPlatform && matchesStatus && matchesSearch
  })
})
const pageRows = computed(() => filteredRows.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value))
const configuredCount = computed(() => rows.value.filter((row) => Boolean(getDeviceId(row))).length)
const missingCount = computed(() => rows.value.length - configuredCount.value)
const tlsTargetIds = computed(() => rows.value
  .filter((row) => row.platform === 'anthropic' && !isTLSFingerprintEnabled(row))
  .map((row) => row.id))
const allPageSelected = computed(() => pageRows.value.length > 0 && pageRows.value.every((row) => selectedIds.value.has(row.id)))
const somePageSelected = computed(() => pageRows.value.some((row) => selectedIds.value.has(row.id)) && !allPageSelected.value)
const confirmTitle = computed(() => {
  if (confirmAction.value === 'tls') return t('admin.deviceIdentity.enableTls')
  if (confirmAction.value === 'all') return t('admin.deviceIdentity.resetAll')
  if (confirmAction.value === 'one') return t('admin.deviceIdentity.resetOne')
  return t('admin.deviceIdentity.generateMissing')
})
const confirmMessage = computed(() => {
  if (confirmAction.value === 'tls') return t('admin.deviceIdentity.confirmEnableTls', { count: tlsTargetIds.value.length })
  if (confirmAction.value === 'all') return t('admin.deviceIdentity.confirmResetAll', { count: rows.value.length })
  if (confirmAction.value === 'one') return t('admin.deviceIdentity.confirmResetOne', { name: pendingResetRow.value?.name || `#${pendingResetRow.value?.id ?? ''}` })
  return t('admin.deviceIdentity.confirmGenerateMissing')
})

function platformLabel(platform: string) {
  return platform === 'anthropic' ? 'Claude' : 'OpenAI'
}

async function loadAccounts() {
  loading.value = true
  try {
    rows.value = await deviceIdentityAPI.list({
      platform: platformFilter.value === 'all' ? undefined : platformFilter.value,
      search: search.value.trim() || undefined
    })
    selectedIds.value = new Set()
  } catch (error) {
    appStore.showError(error instanceof Error ? error.message : t('admin.deviceIdentity.loadFailed'))
  } finally {
    loading.value = false
  }
}

const reloadDebounced = useDebounceFn(() => {
  page.value = 1
  void loadAccounts()
}, 300)

watch([search, platformFilter, statusFilter], reloadDebounced)
watch(filteredRows, () => {
  const maxPage = Math.max(1, Math.ceil(filteredRows.value.length / pageSize.value))
  if (page.value > maxPage) page.value = maxPage
})

function handlePageSizeChange(value: number) {
  pageSize.value = value
  page.value = 1
}

function toggleRow(id: number) {
  const next = new Set(selectedIds.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  selectedIds.value = next
}

function togglePageSelection(event: Event) {
  const checked = (event.target as HTMLInputElement).checked
  const next = new Set(selectedIds.value)
  for (const row of pageRows.value) {
    if (checked) next.add(row.id)
    else next.delete(row.id)
  }
  selectedIds.value = next
}

function copyDeviceId(value: string) {
  void navigator.clipboard?.writeText(value)
  appStore.showSuccess(t('admin.deviceIdentity.copied'))
}

function resetOne(row: DeviceIdentityAccount) {
  if (busy.value) return
  pendingResetRow.value = row
  confirmAction.value = 'one'
}

async function setTLSFingerprint(row: DeviceIdentityAccount, enabled: boolean) {
  if (busy.value || row.platform !== 'anthropic') return
  busy.value = true
  try {
    await deviceIdentityAPI.setTLSFingerprint(row.id, enabled)
    appStore.showSuccess(enabled ? t('admin.deviceIdentity.enabled') : t('admin.deviceIdentity.disabled'))
    await loadAccounts()
  } catch (error) {
    appStore.showError(error instanceof Error ? error.message : t('admin.deviceIdentity.actionFailed'))
  } finally {
    busy.value = false
  }
}

function cancelConfirmation() {
  confirmAction.value = null
  pendingResetRow.value = null
}

async function runConfirmedAction() {
  const action = confirmAction.value
  const resetRow = pendingResetRow.value
  cancelConfirmation()
  if (!action || busy.value) return
  busy.value = true
  try {
    if (action === 'tls') {
      progress.value = { done: 0, total: tlsTargetIds.value.length, success: 0, failed: 0 }
      const result = await deviceIdentityAPI.enableTLSFingerprintMany(tlsTargetIds.value, {
        onProgress: (next) => {
          progress.value = next
        }
      })
      appStore.showSuccess(t('admin.deviceIdentity.tlsResult', result))
    } else if (action === 'one' && resetRow) {
      await deviceIdentityAPI.reset(resetRow.id)
      appStore.showSuccess(t('admin.deviceIdentity.resetSuccess', { name: resetRow.name }))
    } else if (action === 'missing' || action === 'all') {
      const result = await deviceIdentityAPI.ensure(action === 'all')
      appStore.showSuccess(t('admin.deviceIdentity.ensureResult', {
        updated: result.updated,
        skipped: result.skipped,
        failed: result.failed
      }))
    }
    progress.value = { done: 0, total: 0, success: 0, failed: 0 }
    await loadAccounts()
  } catch (error) {
    appStore.showError(error instanceof Error ? error.message : t('admin.deviceIdentity.actionFailed'))
  } finally {
    busy.value = false
  }
}

onMounted(() => {
  void loadAccounts()
})
</script>
