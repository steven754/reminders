const { request } = require('../../utils/request')

Page({
  data: {
    username: '',
    password: '',
    loading: false,
  },

  onUsernameInput(event) { this.setData({ username: event.detail.value }) },
  onPasswordInput(event) { this.setData({ password: event.detail.value }) },

  async submit() {
    const username = this.data.username.trim()
    const password = this.data.password
    if (!username || !password) {
      wx.showToast({ title: '请输入用户名和密码', icon: 'none' })
      return
    }
    this.setData({ loading: true })
    try {
      const result = await request({
        url: '/api/auth/login',
        method: 'POST',
        data: { username, password },
      })
      wx.setStorageSync('reminders_token', result.token)
      wx.setStorageSync('reminders_user', result.user)
      getApp().globalData.user = result.user
      wx.reLaunch({ url: '/pages/index/index' })
    } catch (err) {
      wx.showToast({ title: err.message || '登录失败', icon: 'none' })
    } finally {
      this.setData({ loading: false })
    }
  },
})
