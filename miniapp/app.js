App({
  globalData: {
    user: null,
  },
  onLaunch() {
    const user = wx.getStorageSync('reminders_user')
    if (user) this.globalData.user = user
  },
})
