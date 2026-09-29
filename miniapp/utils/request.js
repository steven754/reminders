const config = require('./config')

function request(options) {
  const token = wx.getStorageSync('reminders_token')
  const header = Object.assign({ 'content-type': 'application/json' }, options.header || {})
  if (token) header.Authorization = `Bearer ${token}`

  return new Promise((resolve, reject) => {
    wx.request({
      url: `${config.baseUrl}${options.url}`,
      method: options.method || 'GET',
      data: options.data,
      header,
      timeout: 30000,
      success(res) {
        const body = res.data || {}
        if (res.statusCode === 401) {
          wx.removeStorageSync('reminders_token')
          wx.removeStorageSync('reminders_user')
          if (getCurrentPages().some(page => page.route !== 'pages/login/login')) {
            wx.reLaunch({ url: '/pages/login/login' })
          }
          reject(new Error(body.message || '登录已失效'))
          return
        }
        if (res.statusCode >= 200 && res.statusCode < 300 && body.code === 0) {
          resolve(body.data)
          return
        }
        reject(new Error(body.message || `请求失败（${res.statusCode}）`))
      },
      fail(err) {
        reject(new Error(err.errMsg || '网络请求失败'))
      },
    })
  })
}

module.exports = { request }
