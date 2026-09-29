const { request } = require('../../utils/request')
const { dateValue, timeValue, localISO, splitISO, defaultStart } = require('../../utils/time')

const repeatOptions = [
  { value: 'none', label: '不重复' },
  { value: 'daily', label: '每天' },
  { value: 'weekly', label: '每周' },
  { value: 'monthly', label: '每月' },
  { value: 'yearly', label: '每年' },
  { value: 'cron', label: '自定义 Cron' },
]
const timeModeOptions = [
  { value: 'point', label: '时间点' },
  { value: 'range', label: '时间段（按间隔）' },
]
const intervalUnits = [
  { value: 'hours', label: '小时', seconds: 3600 },
  { value: 'minutes', label: '分钟', seconds: 60 },
  { value: 'seconds', label: '秒', seconds: 1 },
]
const weekdays = [
  { value: '1', label: '周一' }, { value: '2', label: '周二' }, { value: '3', label: '周三' },
  { value: '4', label: '周四' }, { value: '5', label: '周五' }, { value: '6', label: '周六' }, { value: '0', label: '周日' },
]
const months = Array.from({ length: 12 }, (_, index) => ({ value: String(index + 1), label: `${index + 1}月` }))
const monthDays = Array.from({ length: 31 }, (_, index) => index + 1)
const calendarModeOptions = [
  { value: 'date', label: '按几号' },
  { value: 'weekday', label: '按星期几' },
]

function pad(value) { return String(value).padStart(2, '0') }
function intervalUnitOptions(data) {
  const duration = rangeDurationSeconds(data)
  return intervalUnits.filter(unit => Math.min(Math.floor(Math.max(0, duration - 1) / unit.seconds), unit.value === 'hours' ? 23 : 59) >= 1)
}
function rangeDurationSeconds(data) {
  if (!data.startTime || !data.rangeEndTime) return 0
  const [startHour, startMinute] = data.startTime.split(':').map(Number)
  const [endHour, endMinute] = data.rangeEndTime.split(':').map(Number)
  return Math.max(0, endHour * 3600 + endMinute * 60 - startHour * 3600 - startMinute * 60)
}
function intervalValueOptions(data) {
  const unit = intervalUnits.find(item => item.value === data.intervalUnit) || intervalUnits[2]
  const max = Math.min(Math.floor(Math.max(0, rangeDurationSeconds(data) - 1) / unit.seconds), unit.value === 'hours' ? 23 : 59)
  return Array.from({ length: max }, (_, index) => index + 1)
}
function intervalSeconds(data) {
  const unit = intervalUnits.find(item => item.value === data.intervalUnit) || intervalUnits[2]
  return Number(data.intervalValue || 0) * unit.seconds
}
function buildGeneratedCron(data) {
  if (data.repeatRule === 'none' || data.repeatRule === 'cron' || !data.startTime) return ''
  const [hour, minute] = data.startTime.split(':').map(Number)
  let hourField = String(hour)
  if (data.timeMode === 'range') {
    const [endHour, endMinute] = (data.rangeEndTime || '').split(':').map(Number)
    const step = intervalSeconds(data)
    const start = hour * 3600 + minute * 60
    const end = endHour * 3600 + endMinute * 60
    if (!Number.isInteger(endHour) || end <= start || step <= 0 || step >= end - start) return ''
    let dom = '*'
    let month = '*'
    let dow = '*'
    if (data.repeatRule === 'weekly') dow = data.selectedWeekdays.length ? data.selectedWeekdays.join(',') : '*'
    if (data.repeatRule === 'monthly') {
      if (data.calendarMode === 'date') dom = String(data.monthDay)
      else dow = data.selectedWeekdays.length ? data.selectedWeekdays.join(',') : '*'
    }
    if (data.repeatRule === 'yearly') {
      month = data.selectedMonths.length ? data.selectedMonths.join(',') : '*'
      if (data.calendarMode === 'date') dom = String(data.monthDay)
      else dow = data.selectedWeekdays.length ? data.selectedWeekdays.join(',') : '*'
    }
    const base = `0 ${minute} ${hour} ${dom} ${month} ${dow}`
    return `@interval/v1|step=${step}|start=${start}|end=${end}|base=${base}`
  }
  let dom = '*'
  let month = '*'
  let dow = '*'
  if (data.repeatRule === 'weekly') dow = data.selectedWeekdays.length ? data.selectedWeekdays.join(',') : '*'
  if (data.repeatRule === 'monthly') {
    if (data.calendarMode === 'date') dom = String(data.monthDay)
    else dow = data.selectedWeekdays.length ? data.selectedWeekdays.join(',') : '*'
  }
  if (data.repeatRule === 'yearly') {
    month = data.selectedMonths.length ? data.selectedMonths.join(',') : '*'
    if (data.calendarMode === 'date') dom = String(data.monthDay)
    else dow = data.selectedWeekdays.length ? data.selectedWeekdays.join(',') : '*'
  }
  return `${minute} ${hourField} ${dom} ${month} ${dow}`
}

function parseIntervalExpression(expr) {
  if (!expr || !expr.startsWith('@interval/v1|')) return null
  const fields = Object.fromEntries(expr.split('|').slice(1).map(part => part.split('=')))
  const start = Number(fields.start)
  const end = Number(fields.end)
  const step = Number(fields.step)
  if (!Number.isInteger(start) || !Number.isInteger(end) || !Number.isInteger(step)) return null
  const unit = [...intervalUnits].reverse().find(item => step % item.seconds === 0) || intervalUnits[2]
  return {
    timeMode: 'range',
    rangeEndTime: `${pad(Math.floor(end / 3600))}:${pad(Math.floor((end % 3600) / 60))}`,
    intervalUnit: unit.value,
    intervalValue: step / unit.seconds,
  }
}

function recipientRowsFor(channel, rows) {
  const bindings = (channel.bindings || []).filter(item => item.status === 'active')
  const used = new Set(rows.filter(row => row.id).map(row => row.id))
  return rows.map(row => {
    const options = bindings.filter(binding => binding.id === row.id || !used.has(binding.id))
    return { id: row.id || 0, options, index: Math.max(0, options.findIndex(binding => binding.id === row.id)) }
  })
}

function validateReminderDraft(data) {
  if (!data.title || !data.title.trim()) return '请输入提醒标题'
  if (!data.startDate || !data.startTime) return '请选择开始时间'
  if (data.repeatRule === 'cron' && !data.cronExpr.trim()) return '请输入 Cron 表达式'
  if (data.repeatRule !== 'none' && data.repeatRule !== 'cron' && data.timeMode === 'range' && !data.generatedCron) return '间隔必须小于时间段总长'
  if (data.repeatRule !== 'none' && data.endDate && data.endTime) {
    const start = new Date(`${data.startDate}T${data.startTime}:00`)
    const end = new Date(`${data.endDate}T${data.endTime}:00`)
    if (end <= start) return '结束时间必须晚于开始时间'
  }
  return ''
}

function buildReminderPayload(data) {
  return {
    title: data.title.trim(),
    notes: data.notes.trim(),
    list_id: data.listId,
    priority: 0,
    due_at: localISO(data.startDate, data.startTime),
    end_at: data.repeatRule !== 'none' && data.endDate && data.endTime ? localISO(data.endDate, data.endTime) : null,
    all_day: false,
    repeat_rule: data.repeatRule,
    cron_expr: data.repeatRule === 'cron' ? data.cronExpr.trim() : data.generatedCron,
    calendar: 'solar',
    channels: data.selectedChannels.length ? data.selectedChannels : ['inapp'],
    channel_targets: data.channelTargets,
  }
}

Page({
  data: {
    id: '',
    title: '',
    notes: '',
    startDate: '',
    startTime: '',
    endDate: '',
    endTime: '',
    repeatRule: 'none',
    repeatLabel: '不重复',
    cronExpr: '',
    repeatOptions,
    repeatIndex: 0,
    timeModeOptions,
    timeMode: 'point',
    timeModeLabel: '时间点',
    timeModeIndex: 0,
    intervalUnitOptions: intervalUnits.slice(1),
    intervalUnit: 'minutes',
    intervalUnitLabel: '分钟',
    intervalUnitIndex: 0,
    intervalValueOptions: Array.from({ length: 30 }, (_, index) => index + 1),
    intervalValue: 30,
    intervalValueIndex: 29,
    rangeEndTime: '20:30',
    weekdays,
    selectedWeekdays: ['1'],
    months,
    selectedMonths: ['1'],
    calendarModeOptions,
    calendarMode: 'date',
    calendarModeLabel: '按几号',
    monthDays,
    monthDay: 1,
    monthDayIndex: 0,
    generatedCron: '',
    listId: 0,
    lists: [],
    listIndex: 0,
    channels: [],
    selectedChannels: ['inapp'],
    channelTargets: {},
    recipientDrafts: {},
    loading: false,
    saving: false,
  },

  onLoad(options) {
    const start = defaultStart()
    this.setData({ id: options.id || '', startDate: start.date, startTime: start.time })
    this.refreshIntervalOptions({ startTime: start.time })
    this.loadBaseData(options.id)
  },

  async loadBaseData(id) {
    this.setData({ loading: true })
    try {
      const [lists, channelStatuses] = await Promise.all([
        request({ url: '/api/reminder/lists' }),
        request({ url: '/api/reminder/channels' }),
      ])
      const data = { lists: lists || [], channels: (channelStatuses || []).map(channel => ({ ...channel, recipientRows: [] })) }
      if (!id && lists && lists.length) {
        const defaultIndex = lists.findIndex(item => item.is_default)
        data.listIndex = defaultIndex >= 0 ? defaultIndex : 0
        data.listId = lists[data.listIndex].id
      }
      this.setData(data)
      if (id) await this.loadReminder(id)
    } catch (err) {
      wx.showToast({ title: err.message || '读取数据失败', icon: 'none' })
    } finally {
      this.setData({ loading: false })
    }
  },

  async loadReminder(id) {
    const item = await request({ url: `/api/reminder/items/${id}` })
    const start = splitISO(item.due_at)
    const end = splitISO(item.end_at)
    const repeatIndex = repeatOptions.findIndex(option => option.value === item.repeat_rule)
    const listIndex = this.data.lists.findIndex(list => list.id === item.list_id)
    this.setData({
      title: item.title || '',
      notes: item.notes || '',
      startDate: start.date,
      startTime: start.time,
      endDate: end.date,
      endTime: end.time,
      repeatRule: item.repeat_rule || 'none',
      repeatLabel: repeatIndex >= 0 ? repeatOptions[repeatIndex].label : '不重复',
      repeatIndex: repeatIndex >= 0 ? repeatIndex : 0,
      cronExpr: item.cron_expr || '',
      listId: item.list_id,
      listIndex: listIndex >= 0 ? listIndex : 0,
      selectedChannels: item.channels && item.channels.length ? item.channels : ['inapp'],
      channelTargets: item.channel_targets || {},
    }, () => this.syncRecipientRows())
    this.refreshGeneratedCron({
      repeatRule: item.repeat_rule || 'none',
      startTime: start.time,
      cronExpr: item.cron_expr || '',
    })
    const parsedInterval = parseIntervalExpression(item.cron_expr || '')
    if (parsedInterval) this.refreshIntervalOptions(parsedInterval)
  },

  onTitleInput(event) { this.setData({ title: event.detail.value }) },
  onNotesInput(event) { this.setData({ notes: event.detail.value }) },
  onStartDateChange(event) { this.setData({ startDate: event.detail.value }) },
  onStartTimeChange(event) {
    const startTime = event.detail.value
    this.setData({ startTime })
    this.refreshIntervalOptions({ startTime })
  },
  onEndDateChange(event) { this.setData({ endDate: event.detail.value }) },
  onEndTimeChange(event) { this.setData({ endTime: event.detail.value }) },
  onCronInput(event) { this.setData({ cronExpr: event.detail.value }) },

  onTimeModeChange(event) {
    const index = Number(event.detail.value)
    const option = timeModeOptions[index]
    this.setData({ timeModeIndex: index, timeMode: option.value, timeModeLabel: option.label })
    this.refreshIntervalOptions({ timeMode: option.value })
  },
  onRangeEndTimeChange(event) {
    this.setData({ rangeEndTime: event.detail.value })
    this.refreshIntervalOptions({ rangeEndTime: event.detail.value })
  },
  onIntervalUnitChange(event) {
    const index = Number(event.detail.value)
    const option = this.data.intervalUnitOptions[index]
    if (option) this.refreshIntervalOptions({ intervalUnit: option.value })
  },
  onIntervalValueChange(event) {
    const index = Number(event.detail.value)
    this.refreshIntervalOptions({ intervalValue: this.data.intervalValueOptions[index] })
  },
  onWeekdayChange(event) {
    const selectedWeekdays = event.detail.value
    this.setData({ selectedWeekdays })
    this.refreshIntervalOptions({ selectedWeekdays })
  },
  onMonthChange(event) {
    const selectedMonths = event.detail.value.sort((a, b) => Number(a) - Number(b))
    this.setData({ selectedMonths })
    this.refreshIntervalOptions({ selectedMonths })
  },
  onCalendarModeChange(event) {
    const index = Number(event.detail.value)
    const option = calendarModeOptions[index]
    this.setData({ calendarModeIndex: index, calendarMode: option.value, calendarModeLabel: option.label })
    this.refreshIntervalOptions({ calendarMode: option.value })
  },
  onMonthDayChange(event) {
    const index = Number(event.detail.value)
    this.setData({ monthDayIndex: index, monthDay: monthDays[index] })
    this.refreshIntervalOptions({ monthDay: monthDays[index] })
  },

  onRepeatChange(event) {
    const index = Number(event.detail.value)
    const option = repeatOptions[index]
    this.setData({ repeatIndex: index, repeatRule: option.value, repeatLabel: option.label })
    if (option.value !== 'cron') this.setData({ cronExpr: '' })
    if (option.value === 'none') this.setData({ endDate: '', endTime: '' })
    this.refreshIntervalOptions({ repeatRule: option.value, cronExpr: '' })
  },

  refreshIntervalOptions(overrides) {
    const next = Object.assign({}, this.data, overrides || {})
    let units = intervalUnitOptions(next)
    if (!units.length) units = intervalUnits.slice(2)
    const unit = units.find(item => item.value === next.intervalUnit) || units[0]
    const values = intervalValueOptions(Object.assign({}, next, { intervalUnit: unit.value }))
    const value = values.length ? Math.min(Math.max(1, Number(next.intervalValue || 1)), values[values.length - 1]) : 0
    this.setData({
      intervalUnitOptions: units,
      intervalUnit: unit.value,
      intervalUnitLabel: unit.label,
      intervalUnitIndex: Math.max(0, units.findIndex(item => item.value === unit.value)),
      intervalValueOptions: values,
      intervalValue: value,
      intervalValueIndex: Math.max(0, values.indexOf(value)),
      generatedCron: buildGeneratedCron(Object.assign({}, next, { intervalUnit: unit.value, intervalValue: value })),
    })
  },

  refreshGeneratedCron(overrides) {
    const next = Object.assign({}, this.data, overrides || {})
    const generatedCron = buildGeneratedCron(next)
    this.setData({ generatedCron })
  },

  onListChange(event) {
    const listIndex = Number(event.detail.value)
    const list = this.data.lists[listIndex]
    this.setData({ listIndex, listId: list ? list.id : 0 })
  },

  onChannelChange(event) {
    const selectedChannels = event.detail.value.length ? event.detail.value : ['inapp']
    const channelTargets = { ...this.data.channelTargets }
    Object.keys(channelTargets).forEach(channel => {
      if (!selectedChannels.includes(channel)) delete channelTargets[channel]
    })
    this.setData({ selectedChannels, channelTargets }, () => this.syncRecipientRows())
  },

  syncRecipientRows() {
    const targets = this.data.channelTargets || {}
    const channels = this.data.channels.map(channel => {
      const selected = (targets[channel.channel] || []).map(id => ({ id }))
      const emptyCount = (channel.recipientRows || []).filter(row => !row.id).length
      const rows = selected.concat(Array.from({ length: emptyCount }, () => ({ id: 0 })))
      return { ...channel, recipientRows: recipientRowsFor(channel, rows) }
    })
    this.setData({ channels })
  },

  onAddRecipientRow(event) {
    const channelName = event.currentTarget.dataset.channel
    const channels = this.data.channels.map(channel => channel.channel === channelName
      ? { ...channel, recipientRows: recipientRowsFor(channel, [...(channel.recipientRows || []), { id: 0 }]) }
      : channel)
    this.setData({ channels })
  },

  onRecipientChange(event) {
    const channelName = event.currentTarget.dataset.channel
    const rowIndex = Number(event.currentTarget.dataset.rowIndex)
    const optionIndex = Number(event.detail.value)
    const channel = this.data.channels.find(item => item.channel === channelName)
    const row = channel && channel.recipientRows[rowIndex]
    const binding = row && row.options[optionIndex]
    if (!channel || !row || !binding) return
    const rows = channel.recipientRows.map((item, index) => index === rowIndex ? { ...item, id: binding.id } : item)
    const selected = rows.filter(item => item.id).map(item => item.id)
    if (new Set(selected).size !== selected.length) {
      wx.showToast({ title: '接收人不能重复', icon: 'none' })
      return
    }
    const channelTargets = { ...this.data.channelTargets, [channelName]: selected }
    const channels = this.data.channels.map(item => item.channel === channelName
      ? { ...item, recipientRows: recipientRowsFor(item, rows) }
      : item)
    this.setData({ channels, channelTargets })
  },

  onRemoveRecipientRow(event) {
    const channelName = event.currentTarget.dataset.channel
    const rowIndex = Number(event.currentTarget.dataset.rowIndex)
    const channel = this.data.channels.find(item => item.channel === channelName)
    if (!channel || !channel.recipientRows[rowIndex]) return
    const rows = channel.recipientRows.filter((_, index) => index !== rowIndex)
    const channelTargets = { ...this.data.channelTargets, [channelName]: rows.filter(item => item.id).map(item => item.id) }
    const channels = this.data.channels.map(item => item.channel === channelName
      ? { ...item, recipientRows: recipientRowsFor(item, rows) }
      : item)
    this.setData({ channels, channelTargets })
  },

  async onDeleteRecipientRow(event) {
    const channelName = event.currentTarget.dataset.channel
    const rowIndex = Number(event.currentTarget.dataset.rowIndex)
    const channel = this.data.channels.find(item => item.channel === channelName)
    const row = channel && channel.recipientRows[rowIndex]
    if (!channel || !row) return
    try {
      if (row.id) await request({ url: `/api/reminder/channels/${channelName}/bindings/${row.id}`, method: 'DELETE' })
      const rows = channel.recipientRows.filter((_, index) => index !== rowIndex)
      const channelTargets = { ...this.data.channelTargets, [channelName]: rows.filter(item => item.id).map(item => item.id) }
      const channels = this.data.channels.map(item => item.channel === channelName
        ? { ...item, bindings: (item.bindings || []).filter(binding => binding.id !== row.id), recipientRows: recipientRowsFor({ ...item, bindings: (item.bindings || []).filter(binding => binding.id !== row.id) }, rows) }
        : item)
      this.setData({ channels, channelTargets })
    } catch (err) {
      wx.showToast({ title: err.message || '删除接收人失败', icon: 'none' })
    }
  },

  async onTestRecipientRow(event) {
    const channelName = event.currentTarget.dataset.channel
    const rowIndex = Number(event.currentTarget.dataset.rowIndex)
    const channel = this.data.channels.find(item => item.channel === channelName)
    const row = channel && channel.recipientRows[rowIndex]
    const binding = row && (channel.bindings || []).find(item => item.id === row.id && item.status === 'active')
    if (!binding || !binding.target) {
      wx.showToast({ title: '接收者信息不可用，请重新添加', icon: 'none' })
      return
    }
    const validationError = validateReminderDraft(this.data)
    if (validationError) {
      wx.showToast({ title: `${validationError}后再测试`, icon: 'none' })
      return
    }
    wx.showLoading({ title: '发送中…', mask: true })
    try {
      await request({ url: `/api/reminder/channels/${channelName}/test`, method: 'POST', data: { target: binding.target, reminder: buildReminderPayload(this.data) } })
      wx.showToast({ title: '测试发送成功', icon: 'success' })
    } catch (err) {
      wx.showToast({ title: err.message || '测试发送失败', icon: 'none' })
    } finally {
      wx.hideLoading()
    }
  },

  onRecipientDraftInput(event) {
    const channel = event.currentTarget.dataset.channel
    this.setData({ [`recipientDrafts.${channel}`]: event.detail.value })
  },

  async onAddNewRecipient(event) {
    const channelName = event.currentTarget.dataset.channel
    const target = (this.data.recipientDrafts[channelName] || '').trim()
    if (!target) return
    try {
      const binding = await request({ url: `/api/reminder/channels/${channelName}`, method: 'PUT', data: { target } })
      const channelTargets = { ...this.data.channelTargets, [channelName]: [...(this.data.channelTargets[channelName] || []), binding.id] }
      const channels = this.data.channels.map(channel => {
        if (channel.channel !== channelName) return channel
        const nextChannel = { ...channel, bindings: [...(channel.bindings || []), { ...binding, target: binding.target || binding.target_masked }] }
        return { ...nextChannel, recipientRows: recipientRowsFor(nextChannel, [...(channel.recipientRows || []), { id: binding.id }]) }
      })
      this.setData({ channels, channelTargets, [`recipientDrafts.${channelName}`]: '' })
    } catch (err) {
      wx.showToast({ title: err.message || '保存接收人失败', icon: 'none' })
    }
  },

  async save() {
    const validationError = validateReminderDraft(this.data)
    if (validationError) {
      wx.showToast({ title: validationError, icon: 'none' })
      return
    }
    for (const channel of this.data.selectedChannels) {
      if (channel !== 'inapp' && !(this.data.channelTargets[channel] || []).length) {
        const item = this.data.channels.find(entry => entry.channel === channel)
        wx.showToast({ title: `${item ? item.label : channel}至少选择一个接收人`, icon: 'none' })
        return
      }
    }
    this.setData({ saving: true })
    try {
      const payload = buildReminderPayload(this.data)
      if (this.data.id) {
        const item = await request({ url: `/api/reminder/items/${this.data.id}`, method: 'GET' })
        payload.version = item.version
        await request({ url: `/api/reminder/items/${this.data.id}`, method: 'PUT', data: payload })
      } else {
        await request({ url: '/api/reminder/items', method: 'POST', data: payload })
      }
      wx.showToast({ title: '已保存' })
      setTimeout(() => wx.navigateBack(), 500)
    } catch (err) {
      wx.showToast({ title: err.message || '保存失败', icon: 'none' })
    } finally {
      this.setData({ saving: false })
    }
  },
})
