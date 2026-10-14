<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <div class="relative w-full sm:w-64">
            <Icon
              name="search"
              size="md"
              class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400 dark:text-gray-500"
            />
            <input
              v-model="searchQuery"
              type="text"
              :placeholder="t('admin.upstreams.searchPlaceholder')"
              :aria-label="t('admin.upstreams.searchPlaceholder')"
              class="input pl-10"
              @input="handleSearch"
            />
          </div>
          <div class="flex flex-1 flex-wrap items-center justify-end gap-2">
            <button
              type="button"
              class="btn btn-secondary"
              :disabled="loading"
              :title="t('common.refresh')"
              :aria-label="t('common.refresh')"
              @click="loadUpstreams()"
            >
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
            <button type="button" class="btn btn-primary" @click="openCreate">
              <Icon name="plus" size="md" class="mr-2" />
              {{ t('admin.upstreams.create') }}
            </button>
          </div>
        </div>
      </template>

      <template #table>
        <DataTable :columns="columns" :data="upstreams" :loading="loading">
          <template #cell-name="{ row }">
            <span class="font-medium text-gray-900 dark:text-white">{{ row.name }}</span>
          </template>

          <template #cell-base_url="{ row }">
            <code class="code break-all text-xs">{{ row.base_url }}</code>
          </template>

          <template #cell-api_key="{ row }">
            <code v-if="row.api_key_configured" class="code text-xs">{{ row.api_key_masked }}</code>
            <span v-else class="text-xs text-gray-400">{{ t('admin.upstreams.keyNotConfigured') }}</span>
          </template>

          <template #cell-proxy="{ row }">
            <span v-if="row.proxy_id === null" class="text-sm text-gray-500">{{ t('admin.upstreams.noProxy') }}</span>
            <span v-else-if="proxyNameById.has(row.proxy_id)" class="text-sm">{{ proxyNameById.get(row.proxy_id) }}</span>
            <span v-else class="text-sm text-amber-600 dark:text-amber-400">
              #{{ row.proxy_id }} {{ t('admin.upstreams.proxyUnavailable') }}
            </span>
          </template>

          <template #cell-schedule="{ row }">
            <div class="space-y-1 text-xs">
              <span :class="['badge', row.enabled ? 'badge-success' : 'badge-gray']">
                {{ row.enabled ? t('admin.upstreams.enabled') : t('admin.upstreams.disabled') }}
              </span>
              <div class="text-gray-500">{{ t('admin.upstreams.everyMinutes', { n: row.interval_minutes }) }}</div>
              <div v-if="row.enabled && row.next_probe_at" class="text-gray-400">
                {{ t('admin.upstreams.nextProbe') }} {{ formatDateTime(row.next_probe_at) }}
              </div>
            </div>
          </template>

          <template #cell-balance="{ row }">
            <UpstreamBalanceSummary :snapshot="row.probe" :now="now" />
          </template>

          <template #cell-actions="{ row }">
            <div class="flex items-center gap-1">
              <button
                type="button"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-emerald-50 hover:text-emerald-600 disabled:cursor-not-allowed disabled:opacity-50 dark:hover:bg-emerald-900/20 dark:hover:text-emerald-400"
                :disabled="probingIds.has(row.id)"
                :aria-busy="probingIds.has(row.id)"
                :aria-label="t('admin.upstreams.probeNamed', { name: row.name })"
                data-testid="upstream-probe"
                @click="handleProbe(row)"
              >
                <Icon name="refresh" size="sm" :class="{ 'animate-spin': probingIds.has(row.id) }" />
                <span class="text-xs">{{ t('admin.upstreams.probe') }}</span>
              </button>
              <button
                type="button"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700 dark:hover:text-primary-400"
                data-testid="upstream-edit"
                @click="openEdit(row)"
              >
                <Icon name="edit" size="sm" />
                <span class="text-xs">{{ t('common.edit') }}</span>
              </button>
              <button
                type="button"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"
                data-testid="upstream-delete"
                @click="openDelete(row)"
              >
                <Icon name="trash" size="sm" />
                <span class="text-xs">{{ t('common.delete') }}</span>
              </button>
            </div>
          </template>

          <template #empty>
            <EmptyState
              :title="t('admin.upstreams.emptyTitle')"
              :description="t('admin.upstreams.emptyDescription')"
              :action-text="t('admin.upstreams.create')"
              @action="openCreate"
            />
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>

    <BaseDialog
      :show="showForm"
      :title="editingId === null ? t('admin.upstreams.create') : t('admin.upstreams.edit')"
      width="normal"
      @close="closeForm"
    >
      <form id="upstream-form" class="space-y-4" @submit.prevent="handleSubmit">
        <div>
          <label for="upstream-name" class="input-label">{{ t('admin.upstreams.form.name') }}</label>
          <input id="upstream-name" v-model="form.name" type="text" maxlength="100" required class="input" />
        </div>
        <div>
          <label for="upstream-base-url" class="input-label">{{ t('admin.upstreams.form.baseUrl') }}</label>
          <input
            id="upstream-base-url"
            v-model="form.base_url"
            type="text"
            maxlength="500"
            required
            placeholder="https://example.com"
            class="input"
            aria-describedby="upstream-base-url-hint"
          />
          <p id="upstream-base-url-hint" class="input-hint">{{ t('admin.upstreams.form.baseUrlHint') }}</p>
        </div>
        <div>
          <label for="upstream-api-key" class="input-label">{{ t('admin.upstreams.form.apiKey') }}</label>
          <input
            id="upstream-api-key"
            v-model="form.api_key"
            type="password"
            autocomplete="new-password"
            maxlength="2000"
            :required="editingId === null"
            :placeholder="editingId === null ? '' : editingKeyMasked"
            class="input"
            aria-describedby="upstream-api-key-hint"
          />
          <p id="upstream-api-key-hint" class="input-hint">
            {{ editingId === null ? t('admin.upstreams.form.apiKeyHint') : t('admin.upstreams.form.apiKeyKeepHint') }}
          </p>
        </div>
        <div>
          <span id="upstream-proxy-label" class="input-label">{{ t('admin.upstreams.form.proxy') }}</span>
          <ProxySelector v-model="form.proxy_id" :proxies="proxies" />
        </div>
        <div>
          <label for="upstream-interval" class="input-label">{{ t('admin.upstreams.form.interval') }}</label>
          <input
            id="upstream-interval"
            v-model.number="form.interval_minutes"
            type="number"
            min="5"
            max="1440"
            step="1"
            required
            class="input"
            aria-describedby="upstream-interval-hint"
          />
          <p id="upstream-interval-hint" class="input-hint">{{ t('admin.upstreams.form.intervalHint') }}</p>
        </div>
        <div class="flex items-center justify-between gap-3">
          <div>
            <span id="upstream-enabled-label" class="input-label mb-0">{{ t('admin.upstreams.form.enabled') }}</span>
            <p class="input-hint">{{ t('admin.upstreams.form.enabledHint') }}</p>
          </div>
          <Toggle v-model="form.enabled" aria-labelledby="upstream-enabled-label" />
        </div>
      </form>

      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="closeForm">{{ t('common.cancel') }}</button>
          <button type="submit" form="upstream-form" class="btn btn-primary" :disabled="submitting" :aria-busy="submitting">
            {{ editingId === null ? t('common.create') : t('common.update') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <ConfirmDialog
      :show="deleting !== null"
      :title="t('admin.upstreams.delete')"
      :message="t('admin.upstreams.deleteConfirm', { name: deleting?.name ?? '' })"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="confirmDelete"
      @cancel="deleting = null"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { Proxy, Upstream, UpdateUpstreamRequest } from '@/types'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import ProxySelector from '@/components/common/ProxySelector.vue'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'
import UpstreamBalanceSummary from '@/components/admin/upstream/UpstreamBalanceSummary.vue'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()

const DEFAULT_INTERVAL = 15
// 列表静默刷新周期：让定时探测与创建后的首次探测结果无需手动操作即可出现
const AUTO_RELOAD_MS = 30_000
// 创建／改配置后，后端异步探测一次（请求超时 10s），稍后补一次刷新
const FOLLOW_UP_RELOAD_MS = 5_000

const columns = computed<Column[]>(() => [
  { key: 'name', label: t('admin.upstreams.columns.name') },
  { key: 'base_url', label: t('admin.upstreams.columns.baseUrl') },
  { key: 'api_key', label: t('admin.upstreams.columns.apiKey') },
  { key: 'proxy', label: t('admin.upstreams.columns.proxy') },
  { key: 'schedule', label: t('admin.upstreams.columns.schedule') },
  { key: 'balance', label: t('admin.upstreams.columns.balance') },
  { key: 'actions', label: t('admin.upstreams.columns.actions') }
])

const upstreams = ref<Upstream[]>([])
const proxies = ref<Proxy[]>([])
const proxyNameById = computed(() => new Map(proxies.value.map((p) => [p.id, p.name])))
const loading = ref(false)
const searchQuery = ref('')
const pagination = reactive({ page: 1, page_size: getPersistedPageSize(), total: 0 })
const probingIds = reactive(new Set<number>())
const now = ref(Date.now())

const errorMap = computed(() => ({
  UPSTREAM_PROBE_BUSY: t('admin.upstreams.errors.busy'),
  UPSTREAM_PROBE_IDENTITY_CHANGED: t('admin.upstreams.errors.identityChanged'),
  UPSTREAM_PROXY_NOT_FOUND: t('admin.upstreams.errors.proxyNotFound'),
  UPSTREAM_INVALID_BASE_URL: t('admin.upstreams.errors.invalidBaseUrl')
}))
const showApiError = (error: unknown, fallbackKey: string) =>
  appStore.showError(extractApiErrorMessage(error, t(fallbackKey), errorMap.value))

// 列表请求可能晚于手动探测返回：同一上游保留 last_attempt_at 更新的那份快照
const parseTime = (value?: string | null) => (value ? Date.parse(value) : NaN)
function keepNewerProbe(incoming: Upstream): Upstream {
  const local = upstreams.value.find((row) => row.id === incoming.id)
  const localAt = parseTime(local?.probe?.last_attempt_at)
  const incomingAt = parseTime(incoming.probe?.last_attempt_at)
  if (local?.probe && incoming.probe && localAt > incomingAt) {
    return { ...incoming, ...scheduleFromSnapshot(local) }
  }
  return incoming
}
function scheduleFromSnapshot(row: Upstream): Partial<Upstream> {
  const snapshot = row.probe
  if (!snapshot) return {}
  return {
    probe: snapshot,
    last_attempt_at: snapshot.last_attempt_at,
    next_probe_at: snapshot.next_probe_at,
    failure_count: snapshot.failure_count ?? 0,
    last_error: snapshot.last_error ?? ''
  }
}

let listController: AbortController | null = null
async function loadUpstreams(options: { silent?: boolean } = {}) {
  listController?.abort()
  const controller = new AbortController()
  listController = controller
  if (!options.silent) loading.value = true
  try {
    const response = await adminAPI.upstreams.list(
      pagination.page,
      pagination.page_size,
      { search: searchQuery.value.trim() || undefined },
      { signal: controller.signal }
    )
    if (listController !== controller) return
    upstreams.value = response.items.map(keepNewerProbe)
    pagination.total = response.total
  } catch (error) {
    if (controller.signal.aborted) return
    if (!options.silent) showApiError(error, 'admin.upstreams.loadFailed')
  } finally {
    if (listController === controller) {
      loading.value = false
      listController = null
    }
  }
}

async function loadProxies() {
  try {
    proxies.value = await adminAPI.proxies.getAll()
  } catch (error) {
    // 代理列表只用于显示名称与选择，加载失败时退化为显示代理 ID
    console.error('Error loading proxies:', error)
  }
}

let searchTimer: ReturnType<typeof setTimeout> | undefined
function handleSearch() {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    pagination.page = 1
    loadUpstreams()
  }, 300)
}
function handlePageChange(page: number) {
  pagination.page = page
  loadUpstreams()
}
function handlePageSizeChange(size: number) {
  pagination.page_size = size
  pagination.page = 1
  loadUpstreams()
}

let followUpTimer: ReturnType<typeof setTimeout> | undefined
function scheduleFollowUpReload() {
  clearTimeout(followUpTimer)
  followUpTimer = setTimeout(() => loadUpstreams({ silent: true }), FOLLOW_UP_RELOAD_MS)
}

// ---- 手动探测 ----
async function handleProbe(row: Upstream) {
  if (probingIds.has(row.id)) return
  probingIds.add(row.id)
  const startedUpdatedAt = row.updated_at
  try {
    const { snapshot } = await adminAPI.upstreams.probe(row.id)
    const index = upstreams.value.findIndex((item) => item.id === row.id)
    if (index < 0) return
    const current = upstreams.value[index]
    // 探测期间该行已被编辑或重新读取：快照可能属于旧配置，交给列表重新读取
    if (current.updated_at !== startedUpdatedAt) {
      loadUpstreams({ silent: true })
      return
    }
    upstreams.value[index] = { ...current, ...scheduleFromSnapshot({ ...current, probe: snapshot }) }
  } catch (error) {
    showApiError(error, 'admin.upstreams.probeFailed')
  } finally {
    probingIds.delete(row.id)
  }
}

// ---- 创建／编辑 ----
const showForm = ref(false)
const submitting = ref(false)
const editingId = ref<number | null>(null)
const editingOriginal = ref<Upstream | null>(null)
const editingKeyMasked = computed(() =>
  editingOriginal.value?.api_key_configured ? editingOriginal.value.api_key_masked : ''
)
const emptyForm = () => ({
  name: '',
  base_url: '',
  api_key: '',
  proxy_id: null as number | null,
  enabled: true,
  interval_minutes: DEFAULT_INTERVAL as number | string
})
const form = reactive(emptyForm())

function openCreate() {
  Object.assign(form, emptyForm())
  editingId.value = null
  editingOriginal.value = null
  showForm.value = true
}
function openEdit(row: Upstream) {
  Object.assign(form, {
    name: row.name,
    base_url: row.base_url,
    api_key: '',
    proxy_id: row.proxy_id,
    enabled: row.enabled,
    interval_minutes: row.interval_minutes
  })
  editingId.value = row.id
  editingOriginal.value = row
  showForm.value = true
}
function closeForm() {
  showForm.value = false
  form.api_key = ''
}

function validInterval(value: unknown): value is number {
  return typeof value === 'number' && Number.isInteger(value) && value >= 5 && value <= 1440
}

async function handleSubmit() {
  const name = form.name.trim()
  const baseURL = form.base_url.trim()
  const apiKey = form.api_key.trim()
  if (!name || !baseURL) {
    appStore.showError(t('admin.upstreams.errors.required'))
    return
  }
  if (!validInterval(form.interval_minutes)) {
    appStore.showError(t('admin.upstreams.errors.interval'))
    return
  }
  const original = editingOriginal.value
  if (!original && !apiKey) {
    appStore.showError(t('admin.upstreams.errors.apiKeyRequired'))
    return
  }
  submitting.value = true
  try {
    if (!original) {
      const created = await adminAPI.upstreams.create({
        name,
        base_url: baseURL,
        api_key: apiKey,
        proxy_id: form.proxy_id,
        enabled: form.enabled,
        interval_minutes: form.interval_minutes
      })
      appStore.showSuccess(t('admin.upstreams.created'))
      pagination.page = 1
      upstreams.value = [created, ...upstreams.value.filter((row) => row.id !== created.id)]
    } else {
      // 只提交实际变化的字段：空 Key 不轮换，代理未变不重复校验，避免无谓地作废已有快照
      const payload: UpdateUpstreamRequest = {}
      if (name !== original.name) payload.name = name
      if (baseURL !== original.base_url) payload.base_url = baseURL
      if (apiKey) payload.api_key = apiKey
      if (form.proxy_id !== original.proxy_id) payload.proxy_id = form.proxy_id
      if (form.enabled !== original.enabled) payload.enabled = form.enabled
      if (form.interval_minutes !== original.interval_minutes) payload.interval_minutes = form.interval_minutes
      const updated = await adminAPI.upstreams.update(original.id, payload)
      upstreams.value = upstreams.value.map((row) => (row.id === updated.id ? updated : row))
      appStore.showSuccess(t('admin.upstreams.updated'))
    }
    closeForm()
    loadUpstreams({ silent: true })
    scheduleFollowUpReload()
  } catch (error) {
    showApiError(error, original ? 'admin.upstreams.updateFailed' : 'admin.upstreams.createFailed')
  } finally {
    submitting.value = false
  }
}

// ---- 删除 ----
const deleting = ref<Upstream | null>(null)
function openDelete(row: Upstream) {
  deleting.value = row
}
async function confirmDelete() {
  const target = deleting.value
  if (!target) return
  try {
    await adminAPI.upstreams.delete(target.id)
    appStore.showSuccess(t('admin.upstreams.deleted'))
    deleting.value = null
    upstreams.value = upstreams.value.filter((row) => row.id !== target.id)
    loadUpstreams({ silent: true })
  } catch (error) {
    showApiError(error, 'admin.upstreams.deleteFailed')
  }
}

let reloadTimer: ReturnType<typeof setInterval> | undefined
onMounted(() => {
  loadUpstreams()
  loadProxies()
  reloadTimer = setInterval(() => {
    now.value = Date.now()
    loadUpstreams({ silent: true })
  }, AUTO_RELOAD_MS)
})
onBeforeUnmount(() => {
  clearInterval(reloadTimer)
  clearTimeout(searchTimer)
  clearTimeout(followUpTimer)
  listController?.abort()
})
</script>
