<template>
  <div class="page-container animate-fade-in">
    <PageHeader title="AI 药箱" description="管理药品信息，并在需要时与家人共享药箱">
      <template #actions>
        <RouterLink v-if="authStore.isAdmin" to="/admin/configs" class="btn-ghost">
          <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor"><path d="M12 8.5a3.5 3.5 0 1 0 0 7 3.5 3.5 0 0 0 0-7Z" stroke-width="1.8"/><path d="M19.4 15a1.7 1.7 0 0 0 .34 1.88l.06.06-2.83 2.83-.06-.06A1.7 1.7 0 0 0 15 19.4a1.7 1.7 0 0 0-1 .6 1.7 1.7 0 0 0-.4 1.1V21h-4v-.09A1.7 1.7 0 0 0 8.6 19.4a1.7 1.7 0 0 0-1.88.34l-.06.06-2.83-2.83.06-.06A1.7 1.7 0 0 0 4.6 15a1.7 1.7 0 0 0-.6-1 1.7 1.7 0 0 0-1.1-.4H3v-4h.09A1.7 1.7 0 0 0 4.6 8.6a1.7 1.7 0 0 0-.34-1.88l-.06-.06 2.83-2.83.06.06A1.7 1.7 0 0 0 9 4.6a1.7 1.7 0 0 0 1-.6 1.7 1.7 0 0 0 .4-1.1V3h4v.09A1.7 1.7 0 0 0 15.4 4.6a1.7 1.7 0 0 0 1.88-.34l.06-.06 2.83 2.83-.06.06A1.7 1.7 0 0 0 19.4 9c.13.38.35.72.65 1 .3.28.7.42 1.1.4H21v4h-.09A1.7 1.7 0 0 0 19.4 15Z" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/></svg>
          系统开关
        </RouterLink>
      </template>
    </PageHeader>

    <div v-if="errorMsg" class="rounded-2xl border border-rose-500/20 bg-rose-500/[.07] px-4 py-3 text-sm text-rose-600 dark:text-rose-300">
      {{ errorMsg }}
    </div>

    <section class="surface rounded-2xl p-4 sm:p-5">
      <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
        <div class="flex min-w-0 items-center gap-3">
          <span class="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl bg-rose-500/12 text-rose-500">
            <svg class="h-6 w-6" viewBox="0 0 24 24" fill="none" stroke="currentColor"><path d="M10.5 3.5h3a2 2 0 0 1 2 2v2.2l3.4 5.9a4.5 4.5 0 0 1-3.9 6.7H9a4.5 4.5 0 0 1-3.9-6.7l3.4-5.9V5.5a2 2 0 0 1 2-2Z" stroke-width="1.7" stroke-linejoin="round"/><path d="M9 9h6M8 14h8" stroke-width="1.7" stroke-linecap="round"/></svg>
          </span>
          <div class="min-w-0">
            <p class="text-xs font-extrabold uppercase tracking-[.16em] text-rose-500">当前操作对象</p>
            <p class="truncate text-lg font-extrabold">{{ selectedOwner?.username || '我的药箱' }} 的药箱</p>
          </div>
        </div>
        <label class="flex items-center gap-3 text-sm font-semibold">
          <span class="whitespace-nowrap text-muted-foreground">切换药箱</span>
          <select v-model.number="selectedOwnerID" class="input-field min-w-[180px] !w-auto !py-2" :disabled="loading">
            <option v-for="owner in context?.owners || []" :key="owner.id" :value="owner.id">
              {{ owner.id === context?.current_user.id ? '我的药箱' : `${owner.username} 的药箱` }}
            </option>
          </select>
        </label>
      </div>

      <div v-if="!context?.sharing_enabled" class="mt-4 flex flex-col gap-2 rounded-2xl bg-amber-500/[.08] px-4 py-3 text-sm text-amber-700 dark:text-amber-200 sm:flex-row sm:items-center sm:justify-between">
        <span>跨用户操作当前关闭。管理员开启“允许 Medkit 关联用户”后，才能共享药箱。</span>
        <RouterLink v-if="authStore.isAdmin" to="/admin/configs" class="font-bold underline">去开启</RouterLink>
      </div>
      <div v-else class="mt-4 rounded-2xl bg-emerald-500/[.08] px-4 py-3 text-sm text-emerald-700 dark:text-emerald-200">
        当前药箱权限：<strong>{{ permissionLabel(selectedOwner?.permission) }}</strong>。跨用户操作只在 Medkit 内生效，不会改变普通提醒的归属。
      </div>
    </section>

    <div class="grid gap-4 xl:grid-cols-[minmax(0,1fr)_360px]">
      <section class="surface rounded-2xl p-4 sm:p-5">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 class="text-base font-extrabold">药品清单</h2>
            <p class="mt-1 text-xs text-muted-foreground">共 {{ medicines.length }} 项，支持名称、通用名、规格和备注搜索。</p>
          </div>
          <div class="flex w-full items-center gap-2 sm:w-auto">
            <input v-model.trim="search" class="input-field !py-2 sm:w-52" placeholder="搜索药品…" @input="searchMedicines" />
            <button v-if="selectedOwner?.can_edit" class="btn-brand whitespace-nowrap" @click="startCreate"><span class="text-lg leading-none">＋</span> 添加药品</button>
            <span v-else class="whitespace-nowrap rounded-full bg-muted px-3 py-1.5 text-xs font-semibold text-muted-foreground">只读药箱</span>
          </div>
        </div>

        <form v-if="formOpen" class="mt-5 rounded-2xl border border-brand-500/20 bg-brand-500/[.04] p-4" @submit.prevent="saveMedicine">
          <div class="flex items-center justify-between gap-3">
            <h3 class="font-extrabold">{{ editingID ? '编辑药品' : '新增药品' }}</h3>
            <button type="button" class="icon-button" aria-label="关闭" @click="closeForm">×</button>
          </div>
          <div class="mt-4 rounded-2xl border border-violet-500/15 bg-violet-500/[.06] p-3">
            <div class="flex flex-wrap items-center justify-between gap-2"><p class="text-xs font-extrabold text-violet-600 dark:text-violet-300">AI 快速录入</p><span class="text-[11px] text-muted-foreground">把药盒文字粘贴进来，结果仍需确认</span></div>
            <div class="mt-2 flex flex-col gap-2 sm:flex-row"><textarea v-model.trim="aiText" class="input-field min-h-16 resize-y !py-2" placeholder="例如：阿司匹林肠溶片 100mg，30片，有效期 2027-06-30"></textarea><button type="button" class="btn-ghost shrink-0 sm:self-end" :disabled="aiBusy || !aiText" @click="parseWithAI">{{ aiBusy ? '识别中…' : 'AI 整理' }}</button></div>
          </div>
          <div class="mt-4 grid gap-3 sm:grid-cols-2">
            <label class="field-label sm:col-span-2">药品名称 <span>*</span><input v-model.trim="form.name" class="input-field mt-1.5" maxlength="120" placeholder="例如：阿司匹林肠溶片" required /></label>
            <label class="field-label">通用名称<input v-model.trim="form.generic_name" class="input-field mt-1.5" maxlength="120" placeholder="可选" /></label>
            <label class="field-label">规格<input v-model.trim="form.specification" class="input-field mt-1.5" maxlength="120" placeholder="例如：100mg × 30片" /></label>
            <label class="field-label">数量<input v-model.trim="form.quantity" class="input-field mt-1.5" maxlength="40" placeholder="例如：2" /></label>
            <label class="field-label">单位<input v-model.trim="form.unit" class="input-field mt-1.5" maxlength="24" placeholder="盒、瓶、板" /></label>
            <label class="field-label">到期日期<input v-model="form.expiry_date" class="input-field mt-1.5" type="date" /></label>
            <div class="rounded-2xl border border-border bg-muted/30 p-3 sm:col-span-2">
              <label class="flex items-center gap-2 text-sm font-bold"><input v-model="form.reminder_enabled" type="checkbox" class="h-4 w-4 accent-brand-500" />开启到期前提醒</label>
	              <div v-if="form.reminder_enabled" class="mt-3 grid gap-3 sm:grid-cols-2"><label class="field-label">提前天数<input v-model.number="form.reminder_days" class="input-field mt-1.5" type="number" min="1" max="365" /><small class="mt-1 block font-normal text-muted-foreground">例如 7，表示过期前 7 天提醒</small></label><label class="field-label">提醒时间<input v-model="form.reminder_time" class="input-field mt-1.5" type="time" /><small class="mt-1 block font-normal text-muted-foreground">使用药箱所属用户的通知方式</small></label></div>
	              <div v-if="form.reminder_enabled" class="mt-3 border-t border-border/60 pt-3 sm:col-span-2">
	                <p class="field-label">通知方式 <small class="font-normal text-muted-foreground">可多选</small></p>
	                <p class="mt-1 text-[11px] font-normal text-muted-foreground">选择当前药箱所属用户已配置且有接收目标的方式；未配置的方式会置灰。</p>
	                <div v-if="channelStatuses.length" class="mt-2 flex flex-wrap gap-2">
	                  <button v-for="channel in channelStatuses" :key="channel.channel" type="button" class="rounded-xl border px-3 py-2 text-xs font-bold transition" :class="isReminderChannelSelected(channel.channel) ? 'border-brand-500 bg-brand-500/10 text-brand-600 dark:text-brand-300' : 'border-border bg-surface text-muted-foreground'" :disabled="!canSelectChannel(channel)" :title="channelDisabledReason(channel)" @click="toggleReminderChannel(channel.channel)">
	                    <span>{{ channel.label }}</span>
	                    <span class="ml-1 text-[10px] font-semibold">{{ canSelectChannel(channel) ? (isReminderChannelSelected(channel.channel) ? '已选' : '可选') : channelDisabledReason(channel) }}</span>
	                  </button>
                </div>
                <div v-for="channel in reminderRecipientChannels" :key="`recipient-${channel.channel}`" class="mt-3 rounded-xl border border-border/70 bg-surface/70 p-3">
                  <p class="text-xs font-bold">{{ channel.label }}接收者</p>
                  <div v-if="channel.bindings?.length" class="mt-2 grid gap-2 sm:grid-cols-2">
                    <label v-for="binding in activeBindings(channel)" :key="binding.id" class="flex min-w-0 items-center gap-2 rounded-lg bg-muted/60 px-2.5 py-2 text-xs font-semibold">
                      <input type="checkbox" class="h-4 w-4 shrink-0 accent-brand-500" :checked="isReminderRecipientSelected(channel.channel, binding.id)" @change="toggleReminderRecipient(channel.channel, binding.id)" />
                      <span class="truncate">{{ binding.target || binding.target_masked || `接收者 #${binding.id}` }}</span>
                    </label>
                  </div>
                  <p v-else class="mt-1 text-[11px] font-normal text-muted-foreground">接收者由药箱所有者配置；关联用户不能查看或修改接收地址。</p>
                </div>
                <p v-if="!channelStatuses.length" class="mt-2 text-xs font-normal text-amber-600 dark:text-amber-300">通知方式读取失败，请刷新页面后重试。</p>
              </div>
              <p v-else class="mt-2 text-xs text-muted-foreground">关闭后不会为这项药品创建到期提醒。</p>
            </div>
            <label class="field-label sm:col-span-2">备注<textarea v-model.trim="form.notes" class="input-field mt-1.5 min-h-20 resize-y" maxlength="1000" placeholder="用法、存放位置或需要注意的事项"></textarea></label>
          </div>
          <div class="mt-4 flex justify-end gap-2">
            <button type="button" class="btn-ghost" @click="closeForm">取消</button>
            <button class="btn-brand" :disabled="saving">{{ saving ? '保存中…' : '保存药品' }}</button>
          </div>
        </form>

        <div v-if="loading" class="mt-5 space-y-3"><div v-for="i in 3" :key="i" class="h-24 animate-pulse rounded-2xl bg-muted"></div></div>
        <div v-else-if="medicines.length === 0" class="mt-5 rounded-2xl border border-dashed border-border px-5 py-14 text-center text-sm text-muted-foreground">
          这个药箱还没有药品，点击“添加药品”开始记录。
        </div>
        <div v-else class="mt-5 space-y-3">
          <article v-for="medicine in medicines" :key="medicine.id" class="rounded-2xl border border-border p-4 transition hover:bg-muted/30">
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2">
                  <h3 class="truncate font-extrabold">{{ medicine.name }}</h3>
                  <span v-if="medicine.generic_name" class="rounded-full bg-muted px-2 py-0.5 text-[11px] text-muted-foreground">{{ medicine.generic_name }}</span>
                </div>
                <p class="mt-1 text-sm text-muted-foreground">{{ medicine.specification || '未填写规格' }}<span v-if="medicine.quantity || medicine.unit"> · {{ medicine.quantity || '—' }} {{ medicine.unit }}</span></p>
              </div>
              <div v-if="selectedOwner?.can_edit" class="flex shrink-0 gap-1">
                <button class="icon-button !h-8 !w-8" title="编辑" @click="startEdit(medicine)"><svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor"><path d="m4 16-.8 4.8L8 20l11.5-11.5a2.1 2.1 0 0 0-3-3L5 17Z" stroke-width="1.8" stroke-linejoin="round"/><path d="m14.5 7.5 2 2" stroke-width="1.8"/></svg></button>
                <button class="icon-button !h-8 !w-8 text-rose-500" title="删除" @click="removeMedicine(medicine)"><svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor"><path d="M4 7h16M10 11v6m4-6v6M9 7V4h6v3m-9 0 1 13h10l1-13" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg></button>
              </div>
            </div>
            <div class="mt-3 flex flex-wrap items-center gap-2 text-xs">
              <span v-if="medicine.expiry_date" class="rounded-full px-2.5 py-1 font-semibold" :class="expiryClass(medicine.expiry_date)">{{ expiryLabel(medicine.expiry_date) }}</span>
              <span v-if="medicine.reminder_enabled && medicine.expiry_date" class="rounded-full bg-blue-500/10 px-2.5 py-1 font-semibold text-blue-600 dark:text-blue-300">提前 {{ medicine.reminder_days }} 天 · {{ medicine.reminder_time }}</span>
              <span v-if="medicine.reminder_enabled && medicine.reminder_channels?.length" class="rounded-full bg-violet-500/10 px-2.5 py-1 font-semibold text-violet-600 dark:text-violet-300">{{ medicine.reminder_channels.map(channelLabel).join(' · ') }}</span>
              <span v-else-if="medicine.expiry_date" class="rounded-full bg-muted px-2.5 py-1 font-semibold text-muted-foreground">未开启到期提醒</span>
              <span v-if="medicine.notes" class="text-muted-foreground">{{ medicine.notes }}</span>
            </div>
          </article>
        </div>
      </section>

      <aside class="space-y-4">
        <section v-if="authStore.isAdmin" class="surface rounded-2xl p-4 sm:p-5">
          <div class="flex items-start justify-between gap-3"><div><h2 class="text-base font-extrabold">AI 配置</h2><p class="mt-1 text-xs leading-5 text-muted-foreground">管理员配置 OpenAI 兼容接口，密钥加密保存。</p></div><span class="rounded-full bg-violet-500/10 px-2.5 py-1 text-[11px] font-bold text-violet-600 dark:text-violet-300">可选</span></div>
          <form class="mt-4 space-y-3" @submit.prevent="saveAIConfig">
            <label class="flex items-center gap-2 text-sm font-bold"><input v-model="aiConfig.enabled" type="checkbox" class="h-4 w-4 accent-brand-500" />启用 AI 药品识别</label>
            <input v-model.trim="aiConfig.base_url" class="input-field" placeholder="接口地址，例如 https://api.openai.com/v1" />
            <input v-model.trim="aiConfig.model" class="input-field" placeholder="模型名称，例如 gpt-4o-mini" />
            <input v-model.trim="aiConfig.api_key" class="input-field" type="password" autocomplete="new-password" :placeholder="aiConfig.configured ? '已配置，留空表示不修改' : 'API Key'" />
            <button class="btn-brand w-full" :disabled="aiSaving">{{ aiSaving ? '保存中…' : '保存 AI 配置' }}</button>
          </form>
        </section>
        <section class="surface rounded-2xl p-4 sm:p-5">
          <div class="flex items-start justify-between gap-3">
            <div><h2 class="text-base font-extrabold">关联用户</h2><p class="mt-1 text-xs leading-5 text-muted-foreground">只有药箱所有者可以管理关联关系。</p></div>
            <span class="rounded-full bg-brand-500/10 px-2.5 py-1 text-[11px] font-bold text-brand-600 dark:text-brand-300">仅 Medkit</span>
          </div>
          <template v-if="selectedOwner?.can_manage_users && context?.sharing_enabled">
            <form class="mt-4" @submit.prevent="addAccess">
              <label class="field-label">用户名<input v-model.trim="relation.username" class="input-field mt-1.5" maxlength="100" placeholder="输入应用用户名" @input="searchUsers" /></label>
              <div v-if="searchResults.length" class="mt-2 overflow-hidden rounded-xl border border-border bg-surface">
                <button v-for="user in searchResults" :key="user.id" type="button" class="block w-full px-3 py-2 text-left text-sm hover:bg-muted" @click="relation.username = user.username; searchResults = []">{{ user.username }}</button>
              </div>
              <label class="field-label mt-3">权限<select v-model="relation.permission" class="input-field mt-1.5"><option value="view">只读：只能查看药品</option><option value="edit">可编辑：可以新增、修改、删除药品</option></select></label>
              <button class="btn-brand mt-3 w-full" :disabled="relationBusy || !relation.username">{{ relationBusy ? '保存中…' : '＋ 添加关联用户' }}</button>
            </form>
            <div v-if="managedAccess.length" class="mt-5 space-y-2">
              <div v-for="access in managedAccess" :key="access.id" class="flex items-center justify-between gap-3 rounded-xl bg-muted/60 px-3 py-2.5">
                <div class="min-w-0"><p class="truncate text-sm font-bold">{{ access.member_username }}</p><p class="text-[11px] text-muted-foreground">{{ permissionLabel(access.permission) }}</p></div>
                <button class="text-xs font-bold text-rose-500 hover:underline" @click="removeAccess(access)">移除</button>
              </div>
            </div>
            <p v-else class="mt-4 text-xs text-muted-foreground">还没有关联用户。</p>
          </template>
          <p v-else class="mt-4 rounded-xl bg-muted px-3 py-3 text-xs leading-5 text-muted-foreground">切换到自己的药箱后，才可以添加或管理关联用户。</p>
        </section>

        <section class="rounded-2xl border border-blue-500/15 bg-blue-500/[.06] p-4 text-xs leading-5 text-blue-700 dark:text-blue-200">
          <p class="font-extrabold">权限说明</p>
          <p class="mt-1">跨用户访问只读取 Medkit 药品数据。普通提醒、通知渠道、发件人和接收人仍然属于各自用户，不会因为共享药箱而互相可见。</p>
        </section>
      </aside>
    </div>

    <p v-if="toast" class="fixed bottom-5 left-1/2 z-[90] -translate-x-1/2 rounded-xl bg-slate-900 px-4 py-2.5 text-sm font-semibold text-white shadow-xl dark:bg-white dark:text-slate-900">{{ toast }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import PageHeader from '../components/PageHeader.vue'
import { useAuthStore } from '../stores/auth'
import {
  createMedicine, createMedkitAccess, deleteMedicine, deleteMedkitAccess, getMedkitContext,
  getMedkitAIConfig, getMedkitChannelStatuses, getMedicines, parseMedkitText, saveMedkitAIConfig, searchMedkitUsers, updateMedicine,
  type Medicine, type MedicineInput, type MedkitAccess, type MedkitContext,
} from '../api/medkit'
import type { ChannelStatus, ReminderChannel } from '../api/reminder'

const authStore = useAuthStore()
const context = ref<MedkitContext | null>(null)
const selectedOwnerID = ref(0)
const medicines = ref<Medicine[]>([])
const loading = ref(true)
const saving = ref(false)
const relationBusy = ref(false)
const formOpen = ref(false)
const editingID = ref(0)
const errorMsg = ref('')
const toast = ref('')
const search = ref('')
const aiText = ref('')
const aiBusy = ref(false)
const aiSaving = ref(false)
const searchResults = ref<Array<{ id: number; username: string }>>([])
const channelStatuses = ref<ChannelStatus[]>([])
let toastTimer: number | undefined

const emptyForm = (): MedicineInput => ({ owner_id: 0, name: '', generic_name: '', specification: '', quantity: '', unit: '', expiry_date: '', notes: '', reminder_enabled: true, reminder_days: 7, reminder_time: '09:00', reminder_channels: ['inapp'], reminder_channel_targets: {} })
const form = reactive<MedicineInput>(emptyForm())
const relation = reactive<{ username: string; permission: 'view' | 'edit' }>({ username: '', permission: 'view' })
const aiConfig = reactive({ enabled: false, base_url: 'https://api.openai.com/v1', model: 'gpt-4o-mini', api_key: '', configured: false })

const selectedOwner = computed(() => context.value?.owners.find(item => item.id === selectedOwnerID.value))
const managedAccess = computed(() => context.value?.managed_access || [])
const reminderRecipientChannels = computed(() => channelStatuses.value.filter(channel => channel.channel !== 'inapp' && (form.reminder_channels || []).includes(channel.channel)))

function showToast(message: string) {
  toast.value = message
  if (toastTimer) window.clearTimeout(toastTimer)
  toastTimer = window.setTimeout(() => { toast.value = '' }, 2600)
}

function errorMessage(err: any, fallback: string) {
  return err?.response?.data?.message || fallback
}

async function loadContext() {
  try {
    const res = await getMedkitContext()
    if (res.data?.code !== 0) throw new Error(res.data?.message || '读取 Medkit 设置失败')
    context.value = res.data.data
    if (!selectedOwnerID.value || !context.value.owners.some(owner => owner.id === selectedOwnerID.value)) {
      selectedOwnerID.value = context.value.current_user.id
    }
  } catch (err: any) {
    errorMsg.value = errorMessage(err, '读取 Medkit 设置失败')
  }
}

async function loadMedicines() {
  if (!selectedOwnerID.value) return
  loading.value = true
  try {
    const res = await getMedicines(selectedOwnerID.value, search.value)
    if (res.data?.code !== 0) throw new Error(res.data?.message || '读取药品失败')
    medicines.value = res.data.data || []
  } catch (err: any) {
    medicines.value = []
    errorMsg.value = errorMessage(err, '读取药品失败')
  } finally {
    loading.value = false
  }
}

async function loadChannelStatuses() {
  if (!selectedOwnerID.value) return
  try {
    const res = await getMedkitChannelStatuses(selectedOwnerID.value)
    channelStatuses.value = res.data?.data || []
  } catch {
    channelStatuses.value = []
  }
}

async function reload() {
  errorMsg.value = ''
  await loadContext()
  await Promise.all([loadMedicines(), loadChannelStatuses()])
  if (authStore.isAdmin) {
    try {
      const res = await getMedkitAIConfig()
      if (res.data?.code === 0) Object.assign(aiConfig, { ...res.data.data, api_key: '' })
    } catch {}
  }
}

watch(selectedOwnerID, () => {
  closeForm()
  void loadMedicines()
  void loadChannelStatuses()
})

function startCreate() {
  Object.assign(form, emptyForm(), { owner_id: selectedOwnerID.value })
  editingID.value = 0
  formOpen.value = true
}

function startEdit(medicine: Medicine) {
	const reminderChannels = medicine.reminder_channels?.length ? [...medicine.reminder_channels] : ['inapp'] as ReminderChannel[]
	Object.assign(form, {
    owner_id: medicine.owner_id, name: medicine.name, generic_name: medicine.generic_name,
    specification: medicine.specification, quantity: medicine.quantity, unit: medicine.unit,
    expiry_date: medicine.expiry_date, notes: medicine.notes,
    reminder_enabled: medicine.reminder_enabled, reminder_days: medicine.reminder_days || 7, reminder_time: medicine.reminder_time || '09:00',
    reminder_channels: reminderChannels,
    reminder_channel_targets: defaultReminderTargets(reminderChannels, medicine.reminder_channel_targets),
  })
  editingID.value = medicine.id
  formOpen.value = true
}

function channelLabel(channel: string) {
  return channelStatuses.value.find(item => item.channel === channel)?.label || channel
}

function canSelectChannel(channel: ChannelStatus) {
  if (!channel.configured) return false
  if (channel.channel === 'inapp') return true
  return channel.bound && channel.status === 'active'
}

function channelDisabledReason(channel: ChannelStatus) {
  if (!channel.configured) return '未配置'
  if (channel.channel !== 'inapp' && (!channel.bound || channel.status !== 'active')) return '无有效接收目标'
  return '可选'
}

function isReminderChannelSelected(channel: ReminderChannel) {
  return (form.reminder_channels || []).includes(channel)
}

function toggleReminderChannel(channel: ReminderChannel) {
  if (!canSelectChannel(channelStatuses.value.find(item => item.channel === channel) || { channel, configured: false } as ChannelStatus)) return
  const selected = form.reminder_channels || []
  if (selected.includes(channel)) {
    if (selected.length === 1) {
      showToast('至少保留一种通知方式')
      return
    }
    form.reminder_channels = selected.filter(item => item !== channel)
	} else {
		form.reminder_channels = [...selected, channel]
		form.reminder_channel_targets = defaultReminderTargets(form.reminder_channels, form.reminder_channel_targets)
	}
}

function activeBindings(channel: ChannelStatus) {
	return (channel.bindings || []).filter(binding => binding.status === 'active')
}

function defaultReminderTargets(channels: ReminderChannel[], current?: Record<string, number[]>) {
	const next = { ...(current || {}) }
	for (const channel of channels) {
		if (channel === 'inapp' || next[channel]?.length) continue
		const status = channelStatuses.value.find(item => item.channel === channel)
		const ids = status ? activeBindings(status).map(binding => binding.id) : []
		if (ids.length) next[channel] = ids
	}
	return next
}

function isReminderRecipientSelected(channel: string, bindingID: number) {
	return (form.reminder_channel_targets?.[channel] || []).includes(bindingID)
}

function toggleReminderRecipient(channel: string, bindingID: number) {
	const selected = [...(form.reminder_channel_targets?.[channel] || [])]
	if (selected.includes(bindingID)) {
		if (selected.length === 1) {
			showToast('每种通知方式至少保留一个接收者')
			return
		}
		form.reminder_channel_targets = { ...form.reminder_channel_targets, [channel]: selected.filter(id => id !== bindingID) }
		return
	}
	form.reminder_channel_targets = { ...form.reminder_channel_targets, [channel]: [...selected, bindingID] }
}

function closeForm() {
  formOpen.value = false
  editingID.value = 0
  Object.assign(form, emptyForm())
  aiText.value = ''
}

let searchTimer: number | undefined
function searchMedicines() {
  if (searchTimer) window.clearTimeout(searchTimer)
  searchTimer = window.setTimeout(() => { void loadMedicines() }, 250)
}

async function parseWithAI() {
  if (!aiText.value.trim()) return
  aiBusy.value = true
  errorMsg.value = ''
  try {
    const res = await parseMedkitText(aiText.value.trim())
    if (res.data?.code !== 0) throw new Error(res.data?.message || 'AI 识别失败')
    const item = res.data?.data?.medicines?.[0]
    if (!item) throw new Error('AI 没有识别到药品，请换一种描述')
    Object.assign(form, {
      name: item.name || form.name, generic_name: item.generic_name || form.generic_name,
      specification: item.specification || form.specification, quantity: item.quantity || form.quantity,
      unit: item.unit || form.unit, expiry_date: item.expiry_date || form.expiry_date, notes: item.notes || form.notes,
    })
    showToast('AI 已填入结果，请确认后保存')
  } catch (err: any) {
    errorMsg.value = errorMessage(err, 'AI 识别失败')
  } finally {
    aiBusy.value = false
  }
}

async function saveAIConfig() {
  aiSaving.value = true
  try {
    const res = await saveMedkitAIConfig({ enabled: aiConfig.enabled, base_url: aiConfig.base_url, model: aiConfig.model, api_key: aiConfig.api_key || undefined })
    if (res.data?.code !== 0) throw new Error(res.data?.message || '保存 AI 配置失败')
    Object.assign(aiConfig, { ...res.data.data, api_key: '' })
    showToast('AI 配置已保存')
  } catch (err: any) {
    errorMsg.value = errorMessage(err, '保存 AI 配置失败')
  } finally {
    aiSaving.value = false
  }
}

async function saveMedicine() {
  if (!form.name.trim()) return
  saving.value = true
  errorMsg.value = ''
  try {
    form.owner_id = selectedOwnerID.value
    const res = editingID.value ? await updateMedicine(editingID.value, form) : await createMedicine(form)
    if (res.data?.code !== 0) throw new Error(res.data?.message || '保存药品失败')
    showToast(editingID.value ? '药品已更新' : '药品已添加')
    closeForm()
    await loadMedicines()
  } catch (err: any) {
    errorMsg.value = errorMessage(err, '保存药品失败')
  } finally {
    saving.value = false
  }
}

async function removeMedicine(medicine: Medicine) {
  if (!window.confirm(`确定删除“${medicine.name}”吗？`)) return
  try {
    const res = await deleteMedicine(medicine.id)
    if (res.data?.code !== 0) throw new Error(res.data?.message || '删除药品失败')
    showToast('药品已删除')
    await loadMedicines()
  } catch (err: any) {
    errorMsg.value = errorMessage(err, '删除药品失败')
  }
}

async function searchUsers() {
  if (!context.value?.sharing_enabled || relation.username.trim().length < 1) {
    searchResults.value = []
    return
  }
  try {
    const res = await searchMedkitUsers(relation.username.trim())
    searchResults.value = res.data?.code === 0 ? (res.data.data || []) : []
  } catch {
    searchResults.value = []
  }
}

async function addAccess() {
  if (!relation.username.trim()) return
  relationBusy.value = true
  try {
    const res = await createMedkitAccess({ username: relation.username.trim(), permission: relation.permission })
    if (res.data?.code !== 0) throw new Error(res.data?.message || '添加关联失败')
    relation.username = ''
    relation.permission = 'view'
    searchResults.value = []
    showToast('关联用户已保存')
    await loadContext()
  } catch (err: any) {
    errorMsg.value = errorMessage(err, '添加关联失败')
  } finally {
    relationBusy.value = false
  }
}

async function removeAccess(access: MedkitAccess) {
  if (!window.confirm(`确定移除与“${access.member_username}”的药箱关联吗？`)) return
  try {
    const res = await deleteMedkitAccess(access.id)
    if (res.data?.code !== 0) throw new Error(res.data?.message || '移除关联失败')
    showToast('关联已移除')
    await reload()
  } catch (err: any) {
    errorMsg.value = errorMessage(err, '移除关联失败')
  }
}

function permissionLabel(permission?: string) {
  return permission === 'edit' ? '可编辑' : permission === 'owner' ? '所有者' : '只读'
}

function expiryLabel(date: string) {
  const today = new Date()
  today.setHours(0, 0, 0, 0)
  const expiry = new Date(`${date}T00:00:00`)
  const days = Math.ceil((expiry.getTime() - today.getTime()) / 86400000)
  if (days < 0) return `已过期 · ${date}`
  if (days <= 30) return `${days === 0 ? '今天' : `${days} 天后`}到期 · ${date}`
  return `到期 · ${date}`
}

function expiryClass(date: string) {
  const today = new Date(); today.setHours(0, 0, 0, 0)
  const expiry = new Date(`${date}T00:00:00`)
  const days = Math.ceil((expiry.getTime() - today.getTime()) / 86400000)
  return days < 0 ? 'bg-rose-500/12 text-rose-600 dark:text-rose-300' : days <= 30 ? 'bg-amber-500/12 text-amber-700 dark:text-amber-200' : 'bg-emerald-500/12 text-emerald-700 dark:text-emerald-200'
}

onMounted(() => { void reload() })
</script>

<style scoped>
.field-label { @apply block text-xs font-bold text-foreground; }
.field-label span { @apply text-rose-500; }
</style>
