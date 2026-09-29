const { request } = require('../../utils/request')
const { display } = require('../../utils/time')

Page({
  data: {
    items: [],
    view: 'today',
    loading: false,
    emptyText: '今天还没有待办提醒',
  },

  onShow() {
    if (!wx.getStorageSync('reminders_token')) {
      wx.reLaunch({ url: '/pages/login/login' })
      return
    }
    this.loadItems()
  },

  async loadItems() {
    this.setData({ loading: true })
    try {
      const items = await request({ url: `/api/reminder/items?view=${this.data.view}` })
      this.setData({ items: (items || []).map(item => Object.assign({}, item, { displayTime: item.due_at ? display(item.snoozed_until || item.due_at) : '无日期' })) })
    } catch (err) {
      wx.showToast({ title: err.message || '读取提醒失败', icon: 'none' })
    } finally {
      this.setData({ loading: false })
    }
  },

  switchView(event) {
    const view = event.currentTarget.dataset.view
    this.setData({ view, emptyText: view === 'completed' ? '还没有完成记录' : view === 'planned' ? '还没有计划中的提醒' : '今天还没有待办提醒' })
    this.loadItems()
  },

  addReminder() {
    wx.navigateTo({ url: '/pages/edit/edit' })
  },

  editReminder(event) {
    wx.navigateTo({ url: `/pages/edit/edit?id=${event.currentTarget.dataset.id}` })
  },

  async toggleComplete(event) {
    const item = event.currentTarget.dataset.item
    try {
      await request({
        url: `/api/reminder/items/${item.id}/${item.completed_at ? 'restore' : 'complete'}`,
        method: 'POST',
      })
      this.loadItems()
    } catch (err) {
      wx.showToast({ title: err.message || '操作失败', icon: 'none' })
    }
  },

  async deleteReminder(event) {
    const id = event.currentTarget.dataset.id
    const result = await new Promise(resolve => wx.showModal({ title: '删除提醒', content: '确认删除这条提醒吗？', success: resolve }))
    if (!result.confirm) return
    try {
      await request({ url: `/api/reminder/items/${id}`, method: 'DELETE' })
      this.loadItems()
    } catch (err) {
      wx.showToast({ title: err.message || '删除失败', icon: 'none' })
    }
  },

  logout() {
    wx.removeStorageSync('reminders_token')
    wx.removeStorageSync('reminders_user')
    wx.reLaunch({ url: '/pages/login/login' })
  },
})
