<template>
  <div class="recipient-picker">
    <div class="flex items-center justify-between gap-3">
      <div class="min-w-0">
        <p class="text-[11px] font-bold text-muted-foreground">{{ channel.label }}接收人</p>
        <p class="mt-0.5 text-[10px] text-muted-foreground/75">可多选；接收人只保存在当前用户下</p>
      </div>
      <button type="button" class="recipient-add" :class="{ active: open }" :disabled="!channel.configured" @click="addRow">
        <span class="text-base leading-none">＋</span> 添加接收人
      </button>
    </div>

    <div v-if="rowCount" class="mt-2.5 space-y-1.5">
      <div v-for="(_, index) in rowCount" :key="index" class="recipient-row">
        <select class="recipient-select" :value="rowValue(index) || ''" @change="changeRow(index, $event)">
          <option value="">请选择{{ channel.label }}接收人</option>
          <option v-for="binding in optionsForRow(index)" :key="binding.id" :value="binding.id">{{ binding.target || binding.target_masked }}</option>
        </select>
        <button type="button" class="recipient-clear" :disabled="!!busy" aria-label="移出当前提醒" title="移出当前提醒" @click="clearRow(index)">−</button>
        <button type="button" class="recipient-remove" :disabled="!!busy || !rowValue(index)" @click="deleteRow(index)">{{ busy === `delete-${rowValue(index)}` ? '删除中…' : '删除' }}</button>
        <button type="button" class="recipient-test" :disabled="!!busy || !rowValue(index)" @click="testRow(index)">{{ busy === `test-${rowValue(index)}` ? '发送中…' : '测试发送' }}</button>
      </div>
    </div>
    <p v-else class="mt-2 rounded-xl border border-dashed border-amber-500/25 bg-amber-500/[.05] px-3 py-2 text-[10px] leading-4 text-amber-700 dark:text-amber-200">还没有选择接收人，请点击右上角“＋ 添加接收人”。</p>

    <div v-if="open" class="recipient-panel">
      <p class="text-[10px] font-bold text-muted-foreground">新增接收人</p>
      <p class="mt-1 text-[10px] leading-4 text-muted-foreground">已有接收人请直接在上面的每一行下拉选择；已选过的接收人不会重复出现在其他行。</p>
      <div class="mt-3 flex gap-2">
        <input
          v-model.trim="newTarget"
          class="recipient-input"
          :type="channel.channel === 'email' ? 'email' : 'text'"
          :placeholder="placeholder"
          @keyup.enter="addRecipient"
        />
        <button type="button" class="recipient-add-button" :disabled="!newTarget || !!busy" @click="addRecipient">{{ busy === 'add-new' ? '保存中…' : '保存并选择' }}</button>
      </div>
    </div>
    <p v-if="error" class="mt-2 text-[10px] leading-4" :class="feedbackType === 'success' ? 'text-emerald-500' : 'text-rose-500'">{{ error }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { bindChannel, testChannel, unbindChannelTarget, type ChannelStatus, type SaveReminderInput } from '../api/reminder'

const props = withDefaults(defineProps<{
  channel: ChannelStatus
  modelValue?: number[]
  testPayload?: Partial<SaveReminderInput>
}>(), { modelValue: () => [] })

const emit = defineEmits<{
  'update:modelValue': [value: number[]]
  changed: []
}>()

const open = ref(false)
const newTarget = ref('')
const busy = ref('')
const error = ref('')
const feedbackType = ref<'error' | 'success'>('error')
const bindings = computed(() => (props.channel.bindings || []).filter(item => item.status === 'active'))
const emptyRows = ref(0)
const rowCount = computed(() => (props.modelValue?.length || 0) + emptyRows.value)
const placeholder = computed(() => {
  const placeholders: Record<string, string> = {
    email: '接收邮箱，例如 name@example.com',
    sms: '接收手机号',
    feishu: '工作邮箱、手机号或 OpenID',
    qq: 'QQ 用户 OpenID',
    bark: 'Bark 地址，例如 server|device|group',
  }
  return placeholders[props.channel.channel] || '接收地址、Webhook 或 Token'
})

function setSelected(ids: number[]) {
  emit('update:modelValue', [...new Set(ids)])
}

function rowValue(index: number) {
  return props.modelValue?.[index] || 0
}

function optionsForRow(index: number) {
  const current = rowValue(index)
  const usedByOtherRows = new Set((props.modelValue || []).filter((id, rowIndex) => rowIndex !== index && id > 0))
  return bindings.value.filter(binding => binding.id === current || !usedByOtherRows.has(binding.id))
}

function addRow() {
  emptyRows.value += 1
  open.value = true
  feedbackType.value = 'error'
  error.value = ''
}

function changeRow(index: number, event: Event) {
  const id = Number((event.target as HTMLSelectElement).value)
  if (!id) return
  const next = [...(props.modelValue || [])]
  if (index < next.length) next[index] = id
  else {
    next.push(id)
    emptyRows.value = Math.max(0, emptyRows.value - 1)
  }
  setSelected(next)
  feedbackType.value = 'error'
  error.value = ''
}

function clearRow(index: number) {
  const next = [...(props.modelValue || [])]
  if (index >= next.length) {
    emptyRows.value = Math.max(0, emptyRows.value - 1)
    return
  }
  setSelected(next.filter((_, rowIndex) => rowIndex !== index))
  feedbackType.value = 'error'
  error.value = ''
}

async function deleteRow(index: number) {
  const next = [...(props.modelValue || [])]
  if (index >= next.length) {
    emptyRows.value = Math.max(0, emptyRows.value - 1)
    return
  }
  const id = next[index]
  busy.value = `delete-${id}`
  feedbackType.value = 'error'
  error.value = ''
  try {
    await unbindChannelTarget(props.channel.channel, id)
    setSelected(next.filter((_, rowIndex) => rowIndex !== index))
    emit('changed')
  } catch (err: any) {
    error.value = err.response?.data?.message || '删除接收人失败'
  } finally {
    busy.value = ''
  }
}

async function testRow(index: number) {
  const id = rowValue(index)
  const binding = bindings.value.find(item => item.id === id)
  if (!binding?.target) {
    feedbackType.value = 'error'
    error.value = '该接收者信息不可用，请删除后重新添加'
    return
  }
  const payload = props.testPayload
  if (!payload?.title?.trim()) {
    feedbackType.value = 'error'
    error.value = '请先填写提醒标题，再测试发送'
    return
  }
  if (payload.repeat_rule === 'cron' && !payload.cron_expr?.trim()) {
    feedbackType.value = 'error'
    error.value = '请先填写 Cron 表达式，再测试发送'
    return
  }
  if (payload.repeat_rule && payload.repeat_rule !== 'none' && payload.due_at && payload.end_at && new Date(payload.end_at).getTime() <= new Date(payload.due_at).getTime()) {
    feedbackType.value = 'error'
    error.value = '结束时间必须晚于开始时间，再测试发送'
    return
  }
  busy.value = `test-${id}`
  error.value = ''
  try {
    await testChannel(props.channel.channel, binding.target, payload)
    feedbackType.value = 'success'
    error.value = '测试发送成功'
  } catch (err: any) {
    feedbackType.value = 'error'
    error.value = err.response?.data?.message || '测试发送失败'
  } finally {
    busy.value = ''
  }
}

async function addRecipient() {
  if (!newTarget.value || busy.value) return
  busy.value = 'add-new'
  feedbackType.value = 'error'
  error.value = ''
  try {
    const res = await bindChannel(props.channel.channel, newTarget.value)
    const binding = res.data?.data
    if (!binding?.id) throw new Error('保存接收人失败')
    newTarget.value = ''
    if (emptyRows.value > 0) emptyRows.value -= 1
    setSelected([...(props.modelValue || []), binding.id])
    emit('changed')
    open.value = false
  } catch (err: any) {
    error.value = err.response?.data?.message || err.message || '保存接收人失败'
  } finally {
    busy.value = ''
  }
}

</script>

<style scoped>
.recipient-picker { @apply rounded-2xl border border-border/80 bg-muted/35 p-3; }
.recipient-add { @apply inline-flex shrink-0 items-center gap-1 rounded-xl border border-brand-500/25 bg-brand-500/10 px-2.5 py-1.5 text-[10px] font-bold text-brand-600 transition hover:bg-brand-500/15 disabled:cursor-not-allowed disabled:opacity-45 dark:text-brand-300; }
.recipient-add.active { @apply bg-brand-500 text-white; }
.recipient-row { @apply flex min-h-9 flex-wrap items-center gap-2 rounded-xl border border-border/70 bg-surface px-2.5 py-1.5; }
.recipient-select { @apply min-w-0 flex-1 bg-transparent text-xs font-semibold text-foreground outline-none; }
.recipient-clear { @apply flex h-6 w-6 shrink-0 items-center justify-center rounded-lg border border-border text-sm font-bold leading-none text-muted-foreground transition hover:border-brand-500/30 hover:bg-brand-500/10 hover:text-brand-600 disabled:cursor-not-allowed disabled:opacity-40 dark:hover:text-brand-300; }
.recipient-remove { @apply shrink-0 text-[10px] font-bold text-rose-500 transition hover:text-rose-600 disabled:opacity-40; }
.recipient-test { @apply shrink-0 text-[10px] font-bold text-brand-600 transition hover:text-brand-700 disabled:opacity-40 dark:text-brand-300 dark:hover:text-brand-200; }
.recipient-panel { @apply mt-3 rounded-xl border border-brand-500/15 bg-surface/80 p-3; }
.recipient-input { @apply min-w-0 flex-1 rounded-xl border border-border bg-surface px-3 py-2 text-xs outline-none focus:border-brand-500/45 focus:ring-4 focus:ring-brand-500/10; }
.recipient-add-button { @apply shrink-0 rounded-xl bg-brand-500 px-3 py-2 text-[10px] font-bold text-white disabled:opacity-45; }
</style>
