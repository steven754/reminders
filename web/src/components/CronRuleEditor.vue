<template>
  <div class="space-y-3">
    <label class="field-label">重复方式
      <select :value="repeatRule" class="compact-input" @change="changeRule">
        <option value="none">不重复</option>
        <option value="daily">每天</option>
        <option value="weekly">每周</option>
        <option value="monthly">每月</option>
        <option value="yearly">每年</option>
        <option value="cron">自定义 Cron</option>
      </select>
    </label>

    <div v-if="repeatRule !== 'none'" class="space-y-3 rounded-2xl border border-brand-500/15 bg-brand-500/[0.035] p-3 sm:p-4">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <div>
          <p class="text-xs font-bold text-foreground">循环计划</p>
          <p class="mt-1 text-[10px] leading-4 text-muted-foreground">先用中文选规则；系统会自动生成 Cron。复杂规则可切换为自定义。</p>
        </div>
        <code v-if="repeatRule !== 'cron' && generatedExpression" class="rounded-lg bg-muted px-2 py-1 text-[10px] text-muted-foreground">{{ timeMode === 'range' ? summary : generatedExpression }}</code>
      </div>

    <div v-if="repeatRule === 'cron'" class="space-y-2">
      <label class="field-label">自定义 Cron（5/6 段）<input :value="cronExpr" class="compact-input mt-1.5 font-mono" placeholder="30 8-20/2 * * *" maxlength="120" @input="onCustomInput" /></label>
      <p class="text-[10px] leading-4 text-muted-foreground">5 段格式：分 时 日 月 周；6 段格式：秒 分 时 日 月 周。示例：<code>30 8-20/2 * * *</code>。</p>
    </div>

    <template v-else>
      <div class="grid gap-2 sm:grid-cols-2">
        <label class="field-label">时间规则<select v-model="timeMode" class="compact-input" @change="emitGenerated"><option value="point">时间点</option><option value="range">时间段（按间隔）</option></select></label>
        <div v-if="timeMode === 'point'" class="field-label">
          时间
          <div class="mt-1.5 grid grid-cols-2 gap-2"><select v-model.number="pointHour" class="compact-input" @change="applyPointTime"><option v-for="h in hours" :key="h" :value="h">{{ pad(h) }} 时</option></select><select v-model.number="pointMinute" class="compact-input" @change="applyPointTime"><option v-for="m in minutes" :key="m" :value="m">{{ pad(m) }} 分</option></select></div>
        </div>
        <template v-else>
          <div class="field-label">
            开始时间
            <div class="mt-1.5 grid grid-cols-2 gap-2"><select v-model.number="rangeStartHour" class="compact-input" @change="applyRangeTime"><option v-for="h in hours" :key="h" :value="h">{{ pad(h) }} 时</option></select><select v-model.number="rangeStartMinute" class="compact-input" @change="applyRangeTime"><option v-for="m in minutes" :key="m" :value="m">{{ pad(m) }} 分</option></select></div>
          </div>
          <div class="field-label">
            结束时间
            <div class="mt-1.5 grid grid-cols-2 gap-2"><select v-model.number="rangeEndHour" class="compact-input" @change="emitGenerated"><option v-for="h in hours" :key="h" :value="h">{{ pad(h) }} 时</option></select><select v-model.number="rangeEndMinute" class="compact-input" @change="emitGenerated"><option v-for="m in minutes" :key="m" :value="m">{{ pad(m) }} 分</option></select></div>
          </div>
          <label class="field-label">间隔
            <div class="mt-1.5 grid grid-cols-[1fr_1fr] gap-2">
              <select v-model="intervalUnit" class="compact-input mt-0" @change="onIntervalUnitChange"><option v-for="unit in intervalUnits" :key="unit.value" :value="unit.value" :disabled="unitMaxValue(unit) < 1">{{ unit.label }}</option></select>
              <select v-model.number="intervalValue" class="compact-input mt-0" :disabled="!intervalValueOptions.length" @change="onIntervalValueChange"><option v-for="value in intervalValueOptions" :key="value" :value="value">{{ value }}</option></select>
            </div>
            <span class="mt-1 block text-[10px] font-normal text-muted-foreground">最大可选：{{ formatDuration(maxIntervalSeconds) }}（必须小于时间段总长 {{ formatDuration(durationSeconds) }}）</span>
          </label>
        </template>
      </div>

      <div v-if="repeatRule === 'weekly'" class="space-y-1.5">
        <p class="field-label">星期（可多选）</p>
        <div class="flex flex-wrap gap-1.5"><button v-for="day in weekdays" :key="day.value" type="button" class="rule-pill" :class="{ selected: selectedWeekdays.includes(day.value) }" @click="toggleWeekday(day.value)">{{ day.label }}</button></div>
      </div>

      <div v-if="repeatRule === 'monthly' || repeatRule === 'yearly'" class="space-y-2">
        <label v-if="repeatRule === 'yearly'" class="field-label">月份（可多选）</label>
        <div v-if="repeatRule === 'yearly'" class="flex flex-wrap gap-1.5"><button v-for="month in 12" :key="month" type="button" class="rule-pill" :class="{ selected: selectedMonths.includes(month) }" @click="toggleMonth(month)">{{ month }}月</button></div>
        <label class="field-label">日期方式<select v-model="calendarMode" class="compact-input" @change="emitGenerated"><option value="date">按几号</option><option value="weekday">按星期几</option></select></label>
        <div v-if="calendarMode === 'date'" class="grid gap-2 sm:grid-cols-2">
          <label class="field-label">每月几号<select v-model.number="monthDay" class="compact-input" @change="emitGenerated"><option v-for="day in 31" :key="day" :value="day">{{ day }} 号</option></select></label>
          <p class="self-end text-[10px] leading-4 text-muted-foreground">当月没有该日期时，按当月最后一天执行。</p>
        </div>
        <div v-else class="space-y-1.5">
          <p class="field-label">星期（可多选）</p>
          <div class="flex flex-wrap gap-1.5"><button v-for="day in weekdays" :key="day.value" type="button" class="rule-pill" :class="{ selected: selectedWeekdays.includes(day.value) }" @click="toggleWeekday(day.value)">{{ day.label }}</button></div>
          <p class="text-[10px] leading-4 text-muted-foreground">表示每月/每年所选月份中的这些星期几；需要“第几个星期几”时可使用自定义 Cron。</p>
        </div>
      </div>

      <p v-if="rangeError" class="text-[10px] font-semibold text-rose-500">{{ rangeError }}</p>
      <p v-else class="text-[10px] leading-4 text-muted-foreground">{{ summary }}</p>
    </template>

    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'

const props = withDefaults(defineProps<{
  repeatRule: string
  cronExpr?: string
  dueAt?: string | null
}>(), { cronExpr: '', dueAt: null })

const emit = defineEmits<{
  (e: 'update:repeatRule', value: string): void
  (e: 'update:cronExpr', value: string): void
  (e: 'update:dueAt', value: string | null): void
}>()

const hours = Array.from({ length: 24 }, (_, i) => i)
const minutes = Array.from({ length: 60 }, (_, i) => i)
const intervalUnits = [
  { value: 'hours', label: '小时', seconds: 3600 },
  { value: 'minutes', label: '分钟', seconds: 60 },
  { value: 'seconds', label: '秒', seconds: 1 },
]
const weekdays = [
  { value: 1, label: '周一' }, { value: 2, label: '周二' }, { value: 3, label: '周三' },
  { value: 4, label: '周四' }, { value: 5, label: '周五' }, { value: 6, label: '周六' }, { value: 0, label: '周日' },
]

const timeMode = ref<'point' | 'range'>('point')
const pointHour = ref(9)
const pointMinute = ref(0)
const rangeStartHour = ref(8)
const rangeStartMinute = ref(30)
const rangeEndHour = ref(20)
const rangeEndMinute = ref(30)
const intervalValue = ref(30)
const intervalUnit = ref('minutes')
const selectedWeekdays = ref<number[]>([1])
const selectedMonths = ref<number[]>([1])
const calendarMode = ref<'date' | 'weekday'>('date')
const monthDay = ref(1)
const suppressDueWatch = ref(false)

const pad = (value: number) => String(value).padStart(2, '0')
const dueDate = computed(() => props.dueAt ? new Date(props.dueAt) : new Date())
const durationSeconds = computed(() => {
  const start = rangeStartHour.value * 3600 + rangeStartMinute.value * 60
  const end = rangeEndHour.value * 3600 + rangeEndMinute.value * 60
  return Math.max(0, end - start)
})
const intervalUnitSeconds = computed(() => intervalUnits.find(unit => unit.value === intervalUnit.value)?.seconds || 1)
const intervalMaxValue = computed(() => Math.min(Math.floor(Math.max(0, durationSeconds.value - 1) / intervalUnitSeconds.value), unitCap(intervalUnit.value)))
const intervalValueOptions = computed(() => Array.from({ length: intervalMaxValue.value }, (_, index) => index + 1))
const maxIntervalSeconds = computed(() => intervalMaxValue.value * intervalUnitSeconds.value)
const intervalSeconds = computed(() => Math.max(0, Number(intervalValue.value || 0) * intervalUnitSeconds.value))
const rangeError = computed(() => {
  if (timeMode.value !== 'range') return ''
  if (rangeEndHour.value < rangeStartHour.value || (rangeEndHour.value === rangeStartHour.value && rangeEndMinute.value < rangeStartMinute.value)) return '结束时间必须晚于开始时间。'
  if (durationSeconds.value < 60) return '时间段至少需要 1 分钟。'
  if (intervalMaxValue.value < 1) return '当前时间段没有可用的间隔，请先拉大结束时间。'
  if (intervalValue.value < 1) return '间隔必须大于 0。'
  if (intervalSeconds.value >= durationSeconds.value) return `间隔必须小于时间段总长（${formatDuration(durationSeconds.value)}）。`
  return ''
})

function parseNumbers(value: string, min: number, max: number, fallback: number[]) {
  const result = value.split(',').map(x => Number(x)).filter(x => Number.isInteger(x) && x >= min && x <= max)
  return result.length ? result : fallback
}
function setIntervalFromSeconds(seconds: number) {
  const unit = [...intervalUnits].reverse().find(item => seconds % item.seconds === 0) || intervalUnits[2]
  intervalUnit.value = unit.value
  intervalValue.value = Math.max(1, seconds / unit.seconds)
}
function parseExpression(expr: string) {
  const fields = expr.trim().split(/\s+/)
  if (fields.length !== 5) return
  const [minute, hour, dom, month, dow] = fields
  const minuteNumber = Number(minute)
  if (Number.isInteger(minuteNumber) && minuteNumber >= 0 && minuteNumber <= 59) pointMinute.value = minuteNumber
  const hourNumber = Number(hour)
  const range = hour.match(/^(\d+)-(\d+)\/(\d+)$/)
  if (range) {
    timeMode.value = 'range'
    rangeStartHour.value = Number(range[1]); rangeEndHour.value = Number(range[2]); setIntervalFromSeconds((Number(range[3]) || 1) * 3600)
    rangeStartMinute.value = pointMinute.value; rangeEndMinute.value = pointMinute.value
  } else if (Number.isInteger(hourNumber) && hourNumber >= 0 && hourNumber <= 23) {
    pointHour.value = hourNumber
  }
  if (props.repeatRule === 'weekly' || dow !== '*') selectedWeekdays.value = parseNumbers(dow, 0, 7, [dueDate.value.getDay()]).map(x => x === 7 ? 0 : x)
  if (props.repeatRule === 'monthly' || props.repeatRule === 'yearly') {
    if (dom !== '*') { calendarMode.value = 'date'; monthDay.value = Number(dom) || dueDate.value.getDate() }
    else { calendarMode.value = 'weekday'; selectedWeekdays.value = parseNumbers(dow, 0, 7, [dueDate.value.getDay()]).map(x => x === 7 ? 0 : x) }
  }
  if (props.repeatRule === 'yearly') selectedMonths.value = parseNumbers(month, 1, 12, [dueDate.value.getMonth() + 1])
}
function parseIntervalExpression(expr: string) {
  const fields = Object.fromEntries(expr.split('|').slice(1).map(part => part.split('=')))
  const step = Number(fields.step)
  const start = Number(fields.start)
  const end = Number(fields.end)
  if (!Number.isInteger(step) || !Number.isInteger(start) || !Number.isInteger(end)) return false
  timeMode.value = 'range'
  rangeStartHour.value = Math.floor(start / 3600)
  rangeStartMinute.value = Math.floor((start % 3600) / 60)
  rangeEndHour.value = Math.floor(end / 3600)
  rangeEndMinute.value = Math.floor((end % 3600) / 60)
  setIntervalFromSeconds(step)
  return true
}
function initFromProps() {
  const d = dueDate.value
  pointHour.value = d.getHours(); pointMinute.value = d.getMinutes()
  rangeStartHour.value = d.getHours(); rangeStartMinute.value = d.getMinutes()
  rangeEndHour.value = Math.min(23, d.getHours() + 12); rangeEndMinute.value = d.getMinutes()
  intervalValue.value = 30; intervalUnit.value = 'minutes'
  selectedWeekdays.value = [d.getDay()]
  selectedMonths.value = [d.getMonth() + 1]
  monthDay.value = d.getDate()
  timeMode.value = 'point'; calendarMode.value = 'date'
  if (props.cronExpr && props.repeatRule !== 'none') {
    if (!props.cronExpr.startsWith('@interval/v1|') || !parseIntervalExpression(props.cronExpr)) parseExpression(props.cronExpr)
  }
}

function timeFields() {
  if (timeMode.value === 'point') return { minute: pointMinute.value, hour: String(pointHour.value) }
  if (rangeError.value) return null
  return { minute: rangeStartMinute.value, hour: `${rangeStartHour.value}-${rangeEndHour.value}` }
}
const generatedExpression = computed(() => buildExpression(props.repeatRule))
function buildExpression(rule: string) {
  if (!rule || rule === 'none') return ''
  const time = timeFields()
  if (!time) return ''
  let dom = '*'; let month = '*'; let dow = '*'
  if (rule === 'weekly') dow = selectedWeekdays.value.length ? selectedWeekdays.value.join(',') : '*'
  if (rule === 'monthly') {
    if (calendarMode.value === 'date') dom = String(monthDay.value)
    else dow = selectedWeekdays.value.length ? selectedWeekdays.value.join(',') : '*'
  }
  if (rule === 'yearly') {
    month = selectedMonths.value.length ? selectedMonths.value.sort((a, b) => a - b).join(',') : '*'
    if (calendarMode.value === 'date') dom = String(monthDay.value)
    else dow = selectedWeekdays.value.length ? selectedWeekdays.value.join(',') : '*'
  }
  if (timeMode.value === 'range') {
    const start = rangeStartHour.value * 3600 + rangeStartMinute.value * 60
    const end = rangeEndHour.value * 3600 + rangeEndMinute.value * 60
    const base = `${dueDate.value.getSeconds()} ${time.minute} ${rangeStartHour.value} ${dom} ${month} ${dow}`
    return `@interval/v1|step=${intervalSeconds.value}|start=${start}|end=${end}|base=${base}`
  }
  return `${time.minute} ${time.hour} ${dom} ${month} ${dow}`
}
function emitGenerated() {
  if (props.repeatRule === 'cron' || props.repeatRule === 'none') return
  emit('update:cronExpr', generatedExpression.value)
}
function updateDueTime(hour: number, minute: number) {
  const d = dueDate.value
  d.setHours(hour, minute, 0, 0)
  suppressDueWatch.value = true
  emit('update:dueAt', toLocalIso(d))
}
function applyPointTime() {
  updateDueTime(pointHour.value, pointMinute.value)
  emitGenerated()
}
function applyRangeTime() {
  updateDueTime(rangeStartHour.value, rangeStartMinute.value)
  emitGenerated()
}
function onIntervalValueChange() {
  if (intervalMaxValue.value > 0) intervalValue.value = Math.min(Math.max(1, Number(intervalValue.value || 1)), intervalMaxValue.value)
  emitGenerated()
}
function onIntervalUnitChange() {
  if (intervalMaxValue.value > 0) intervalValue.value = Math.min(Math.max(1, Number(intervalValue.value || 1)), intervalMaxValue.value)
  emitGenerated()
}
function unitMaxValue(unit: { seconds: number }) {
  const unitValue = intervalUnits.find(item => item.seconds === unit.seconds)?.value || 'seconds'
  return Math.min(Math.floor(Math.max(0, durationSeconds.value - 1) / unit.seconds), unitCap(unitValue))
}
function unitCap(unit: string) {
  return unit === 'hours' ? 23 : 59
}
function toggleWeekday(day: number) {
  selectedWeekdays.value = selectedWeekdays.value.includes(day) ? selectedWeekdays.value.filter(x => x !== day) : [...selectedWeekdays.value, day].sort()
  emitGenerated()
}
function toggleMonth(month: number) {
  selectedMonths.value = selectedMonths.value.includes(month) ? selectedMonths.value.filter(x => x !== month) : [...selectedMonths.value, month].sort((a, b) => a - b)
  emitGenerated()
}
function onCustomInput(event: Event) {
  emit('update:cronExpr', (event.target as HTMLInputElement).value)
}
function changeRule(event: Event) {
  const rule = (event.target as HTMLSelectElement).value
  emit('update:repeatRule', rule)
  if (rule === 'none') emit('update:cronExpr', '')
  else if (rule !== 'cron') setTimeout(emitGenerated, 0)
}
function toLocalIso(d: Date) {
  const offsetMinutes = -d.getTimezoneOffset(); const sign = offsetMinutes >= 0 ? '+' : '-'; const absOffset = Math.abs(offsetMinutes)
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}:00${sign}${pad(Math.floor(absOffset / 60))}:${pad(absOffset % 60)}`
}
const summary = computed(() => {
  if (!generatedExpression.value) return '请完善时间段或选择至少一个星期/月份。'
  const scope = props.repeatRule === 'daily' ? '每天' : props.repeatRule === 'weekly' ? '每周' : props.repeatRule === 'monthly' ? '每月' : '每年'
  if (timeMode.value === 'range') return `${scope} ${formatClock(rangeStartHour.value, rangeStartMinute.value)} 至 ${formatClock(rangeEndHour.value, rangeEndMinute.value)}，每 ${intervalValue.value} ${intervalUnits.find(unit => unit.value === intervalUnit.value)?.label || ''} 执行`
  return `${scope}执行：${generatedExpression.value}`
})
function formatClock(hour: number, minute: number) { return `${pad(hour)}:${pad(minute)}` }
function formatDuration(seconds: number) {
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const remainder = seconds % 60
  const parts: string[] = []
  if (hours) parts.push(`${hours} 小时`)
  if (minutes) parts.push(`${minutes} 分钟`)
  if (remainder || !parts.length) parts.push(`${remainder} 秒`)
  return parts.join(' ')
}

watch(() => [durationSeconds.value, intervalUnit.value], () => {
  if (intervalMaxValue.value < 1) {
    const available = intervalUnits.find(unit => unitMaxValue(unit) > 0)
    if (available) {
      intervalUnit.value = available.value
      intervalValue.value = Math.min(30, unitMaxValue(available))
    }
  }
  if (intervalMaxValue.value > 0 && intervalValue.value > intervalMaxValue.value) intervalValue.value = intervalMaxValue.value
})

watch(() => [props.repeatRule, props.dueAt], () => {
  if (suppressDueWatch.value) { suppressDueWatch.value = false; return }
  initFromProps()
  if (props.repeatRule !== 'none' && props.repeatRule !== 'cron') setTimeout(emitGenerated, 0)
}, { immediate: true })
</script>

<style scoped>
.field-label { @apply block text-[11px] font-bold text-muted-foreground; }
.compact-input { @apply mt-1.5 h-9 w-full rounded-xl border border-border bg-surface px-3 text-xs font-medium text-foreground outline-none focus:border-brand-500/50; }
.rule-pill { @apply rounded-xl border border-border bg-surface px-3 py-1.5 text-[11px] font-semibold text-muted-foreground transition hover:border-brand-500/30 hover:text-brand-600; }
.rule-pill.selected { @apply border-brand-500/30 bg-brand-500/10 text-brand-600 dark:text-brand-300; }
</style>
