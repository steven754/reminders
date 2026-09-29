function pad(value) {
  return String(value).padStart(2, '0')
}

function dateValue(date) {
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

function timeValue(date) {
  return `${pad(date.getHours())}:${pad(date.getMinutes())}`
}

function localISO(date, time) {
  const offset = -date.getTimezoneOffset()
  const sign = offset >= 0 ? '+' : '-'
  const absolute = Math.abs(offset)
  return `${date}T${time}:00${sign}${pad(Math.floor(absolute / 60))}:${pad(absolute % 60)}`
}

function splitISO(value) {
  if (!value) return { date: '', time: '' }
  const date = new Date(value)
  return { date: dateValue(date), time: timeValue(date) }
}

function display(value) {
  if (!value) return '无日期'
  const date = new Date(value)
  return `${date.getMonth() + 1}月${date.getDate()}日 ${timeValue(date)}`
}

function defaultStart() {
  const date = new Date(Date.now() + 60 * 60 * 1000)
  date.setSeconds(0, 0)
  return { date: dateValue(date), time: timeValue(date) }
}

module.exports = { dateValue, timeValue, localISO, splitISO, display, defaultStart }
