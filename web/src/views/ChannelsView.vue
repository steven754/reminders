<template>
  <div class="mx-auto max-w-5xl">
    <div class="mb-4 sm:mb-7">
      <p class="mb-1 text-xs font-extrabold uppercase tracking-[.18em] text-violet-500 sm:mb-2">随时触达</p>
      <h1 class="font-display text-2xl font-extrabold tracking-tight sm:text-3xl">通知方式</h1>
      <p class="mt-0.5 max-w-2xl text-[13px] leading-5 text-muted-foreground sm:mt-2 sm:text-sm sm:leading-6">站内消息默认可用。这里配置发件服务和机器人；创建提醒时，再为每种方式选择或添加接收人。</p>
    </div>

    <div class="mb-4 rounded-2xl border border-blue-500/15 bg-blue-500/[.06] px-3.5 py-2.5 text-xs leading-5 text-blue-700 dark:text-blue-200 sm:mb-5 sm:px-4 sm:py-3">
      这里主要配置发送方或服务商。创建提醒时，点击对应方式旁边的“＋ 添加接收人”，再选择或填写接收目标。邮件填写邮箱，短信填写手机号，飞书填写工作邮箱、手机号或 OpenID。
    </div>

    <section v-if="authStore.isAdmin" class="mb-4 flex flex-col gap-3 rounded-2xl border border-violet-500/15 bg-violet-500/[.05] p-3.5 sm:mb-5 sm:flex-row sm:items-end sm:p-4">
      <label class="min-w-0 flex-1 text-xs font-extrabold text-foreground">机器人消息来源名称
        <input v-model.trim="notificationBrand" class="bind-input mt-2" maxlength="40" placeholder="例如：我的提醒" />
      </label>
      <button class="bind-button shrink-0" :disabled="busy === 'notification-brand' || !notificationBrand" @click="saveBrand">{{ busy === 'notification-brand' ? '保存中…' : '保存名称' }}</button>
      <p class="text-[11px] leading-5 text-muted-foreground sm:max-w-48">会显示在飞书和 QQ 消息开头，例如【我的提醒】。</p>
    </section>

    <div v-if="loading" class="grid gap-4 md:grid-cols-2">
      <div v-for="n in 4" :key="n" class="h-52 animate-pulse rounded-[24px] bg-surface"></div>
    </div>

    <div v-else-if="loadError" class="flex min-h-[220px] flex-col items-center justify-center gap-3 rounded-[24px] border border-dashed border-rose-500/30 bg-rose-500/[.04] px-6 text-center">
      <svg class="h-10 w-10 text-rose-500/70" viewBox="0 0 24 24" fill="none" stroke="currentColor"><path d="M12 9v4m0 4h.01M10.3 3.8 2.5 17.2A2 2 0 0 0 4.2 20h15.6a2 2 0 0 0 1.7-2.8L13.7 3.8a2 2 0 0 0-3.4 0Z" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>
      <p class="text-sm font-semibold text-rose-500">{{ loadError }}</p>
      <button class="rounded-xl border border-border bg-surface px-4 py-2 text-xs font-bold text-foreground transition hover:bg-muted" @click="load()">重试</button>
    </div>

    <div v-else class="grid gap-4 md:grid-cols-2">
      <article v-for="channel in channels" :key="channel.channel" class="channel-card">
        <div class="flex items-start gap-4">
          <span class="channel-icon" :class="channel.channel" v-html="icon(channel.channel)"></span>
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
              <h2 class="text-base font-extrabold">{{ channel.label }}</h2>
              <span class="status-pill" :class="statusClass(channel)">{{ statusText(channel) }}</span>
            </div>
            <p class="mt-1 text-xs leading-5 text-muted-foreground">{{ channel.description }}</p>
          </div>
        </div>

        <button v-if="authStore.isAdmin && channel.configured && ['feishu', 'qq'].includes(channel.channel)" class="mt-4 w-full rounded-xl border border-violet-500/15 bg-violet-500/[.05] px-3 py-2 text-xs font-bold text-violet-600" @click="openBotSetup(channel.channel)">
          更换机器人 App ID / App Secret
        </button>

        <div v-if="channel.channel === 'inapp'" class="mt-4 sm:mt-5 rounded-2xl bg-emerald-500/[.07] px-4 py-3 text-xs font-semibold text-emerald-600 dark:text-emerald-300">
          ✓ 已启用，提醒会保存在通知中心
        </div>

        <div v-else-if="channel.channel === 'email' && channel.configured" class="mt-4 sm:mt-5">
          <div class="rounded-2xl border border-border bg-muted/45 px-4 py-3">
            <div class="flex items-center justify-between">
              <p class="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">发件邮箱服务已配置</p>
              <span class="h-2.5 w-2.5 rounded-full bg-emerald-500"></span>
            </div>
            <p class="mt-2 text-xs leading-5 text-muted-foreground">接收邮箱不在这里绑定。创建提醒时，在“电子邮件接收人”区域逐行选择或添加。</p>
          </div>
          <p class="mt-2 text-[10px] leading-4 text-muted-foreground">接收邮箱请在创建或编辑提醒时，通过“＋ 添加接收人”逐行选择。</p>
          <button v-if="authStore.isAdmin" class="channel-button mt-3" @click="openEmailSetup(true)">修改发件服务</button>
          <div class="channel-test mt-3">
            <p class="channel-test-help">发送测试需要填写本次接收邮箱，不会保存为提醒接收人。</p>
            <div class="mt-2 flex gap-2">
              <input v-model.trim="testTargets[channel.channel]" class="bind-input min-w-0 flex-1" :type="testInputType(channel.channel)" :placeholder="testPlaceholder(channel.channel)" />
              <button class="channel-button primary shrink-0" :disabled="busy === channel.channel || !testTargets[channel.channel]" @click="test(channel)">{{ busy === channel.channel ? '发送中…' : '发送测试' }}</button>
            </div>
          </div>
        </div>

        <div v-else-if="channel.channel === 'sms' && channel.configured && authStore.isAdmin" class="mt-4 sm:mt-5">
          <div class="rounded-2xl border border-border bg-muted/45 px-4 py-3">
            <p class="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">短信发送服务已配置</p>
            <p class="mt-2 text-xs leading-5 text-muted-foreground">修改时请重新填写短信服务 Webhook；已保存的 Token 不会回显。</p>
          </div>
          <button class="channel-button mt-3" @click="editingProvider = editingProvider === 'sms' ? '' : 'sms'">{{ editingProvider === 'sms' ? '收起修改' : '修改短信服务' }}</button>
          <form v-if="editingProvider === 'sms'" class="mt-3 space-y-3 rounded-2xl border border-violet-500/15 bg-violet-500/[.05] p-4" @submit.prevent="configureProvider('sms')">
            <input v-model.trim="providerConfig.sms.webhook_url" class="bind-input w-full" placeholder="短信服务 Webhook 地址" type="url" />
            <input v-model.trim="providerConfig.sms.webhook_token" class="bind-input w-full" placeholder="可选：Webhook Token" type="password" autocomplete="new-password" />
            <button class="bind-button w-full" :disabled="busy === 'sms-config' || !providerConfig.sms.webhook_url">{{ busy === 'sms-config' ? '保存中…' : '保存修改' }}</button>
          </form>
          <div class="channel-test mt-3">
            <p class="channel-test-help">发送测试需要填写本次接收手机号，不会保存为提醒接收人。</p>
            <div class="mt-2 flex gap-2">
              <input v-model.trim="testTargets[channel.channel]" class="bind-input min-w-0 flex-1" :type="testInputType(channel.channel)" :placeholder="testPlaceholder(channel.channel)" />
              <button class="channel-button primary shrink-0" :disabled="busy === channel.channel || !testTargets[channel.channel]" @click="test(channel)">{{ busy === channel.channel ? '发送中…' : '发送测试' }}</button>
            </div>
          </div>
        </div>

        <div v-else-if="channel.configured && channel.channel !== 'qq'" class="mt-4 sm:mt-5">
          <div class="rounded-2xl border border-border bg-muted/45 px-4 py-3">
            <p class="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">发送服务已可用</p>
            <p class="mt-2 text-xs leading-5 text-muted-foreground">接收地址、Webhook 或 Token 请在创建提醒时，通过“＋ 添加接收人”逐行添加；同一方式可以添加多个。</p>
          </div>
          <div class="channel-test mt-3">
            <p class="channel-test-help">发送测试需要填写本次接收者，不会保存为提醒接收人。</p>
            <div class="mt-2 flex gap-2">
              <input v-model.trim="testTargets[channel.channel]" class="bind-input min-w-0 flex-1" :type="testInputType(channel.channel)" :placeholder="testPlaceholder(channel.channel)" />
              <button class="channel-button primary shrink-0" :disabled="busy === channel.channel || !testTargets[channel.channel]" @click="test(channel)">{{ busy === channel.channel ? '发送中…' : '发送测试' }}</button>
            </div>
          </div>
        </div>

        <div v-else-if="!channel.configured && channel.channel === 'feishu' && authStore.isAdmin" class="mt-4 sm:mt-5 rounded-2xl border border-cyan-500/20 bg-cyan-500/[.06] p-4">
          <p class="text-sm font-extrabold">开通飞书机器人</p>
          <p class="mt-1 text-xs leading-5 text-muted-foreground">填写飞书开放平台的应用凭证。保存后立即生效，无需重启；凭证仅加密保存在本应用中。</p>
          <form class="mt-4 space-y-3" @submit.prevent="configureFeishu">
            <input v-model.trim="feishuConfig.app_id" class="bind-input" placeholder="App ID，例如 cli_xxxxx" autocomplete="off" />
            <input v-model.trim="feishuConfig.app_secret" class="bind-input" type="password" placeholder="App Secret" autocomplete="new-password" />
            <button class="bind-button w-full" :disabled="busy === 'feishu-config' || !feishuConfig.app_id || !feishuConfig.app_secret">{{ busy === 'feishu-config' ? '开通中…' : '保存并开通飞书机器人' }}</button>
          </form>
          <p class="mt-3 text-[10px] leading-4 text-muted-foreground">还需在飞书开放平台启用机器人、设置应用可见范围，并开通“通过邮箱或手机号获取用户 ID”的通讯录权限。</p>
        </div>

        <div v-else-if="!channel.configured && channel.channel === 'email' && authStore.isAdmin" class="provider-form"><p class="text-sm font-extrabold">开通邮件提醒</p><p class="provider-help">先配置用于发出提醒的发件邮箱和授权码；配置完成后，创建提醒时再添加接收邮箱。</p><button class="bind-button mt-4 w-full" @click="openEmailSetup(false)">配置邮件服务</button></div>

        <div v-else-if="!channel.configured && channel.channel === 'sms' && authStore.isAdmin" class="provider-form">
          <p class="text-sm font-extrabold">开通短信提醒</p><p class="provider-help">填写你的短信服务转发地址。系统会向该地址发送手机号、提醒标题、时间和幂等标识。</p>
          <form class="mt-4 space-y-3" @submit.prevent="configureProvider('sms')"><input v-model.trim="providerConfig.sms.webhook_url" class="bind-input" placeholder="短信服务 Webhook 地址" type="url" /><input v-model.trim="providerConfig.sms.webhook_token" class="bind-input" placeholder="可选：Webhook Token" type="password" autocomplete="new-password" /><button class="bind-button w-full" :disabled="busy === 'sms-config' || !providerConfig.sms.webhook_url">{{ busy === 'sms-config' ? '开通中…' : '保存并开通短信' }}</button></form>
        </div>

        <div v-else-if="!channel.configured && channel.channel === 'qq' && authStore.isAdmin" class="provider-form">
          <p class="text-sm font-extrabold">开通 QQ 机器人</p>
          <p class="provider-help">填写 QQ 开放平台的 App ID 和 App Secret。接收人的 OpenID 在创建提醒时填写。</p>
          <form class="mt-4 space-y-4" @submit.prevent="configureProvider('qq')">
            <label class="provider-field">
              <span class="provider-field-label">App ID <small class="provider-badge required">必填</small></span>
              <input v-model.trim="providerConfig.qq.app_id" class="bind-input w-full" placeholder="例如：1903753507" autocomplete="off" />
              <span class="provider-field-help">QQ 开放平台创建机器人后生成的唯一标识，用来识别你的机器人。</span>
            </label>
            <label class="provider-field">
              <span class="provider-field-label">App Secret <small class="provider-badge required">必填</small></span>
              <input v-model.trim="providerConfig.qq.app_secret" class="bind-input w-full" placeholder="请输入 QQ 开放平台的 App Secret" type="password" autocomplete="new-password" />
              <span class="provider-field-help">与 App ID 配套的密钥，只用于向 QQ 官方接口换取访问凭证，请勿分享。</span>
            </label>
            <label class="provider-field">
              <span class="provider-field-label">机器人主页 / 邀请链接 <small class="provider-badge recommended">推荐</small></span>
              <input v-model.trim="providerConfig.qq.bot_link" class="bind-input w-full" type="url" placeholder="例如：https://q.qq.com/..." />
              <span class="provider-field-help">绑定时显示“打开 QQ 机器人”按钮，方便用户进入机器人；不参与消息发送。</span>
            </label>
            <label class="provider-field">
              <span class="provider-field-label">消息 API 地址 <small class="provider-badge optional">可选</small></span>
              <input v-model.trim="providerConfig.qq.api_base" class="bind-input w-full" placeholder="留空使用 https://api.bot.qq.com" />
              <span class="provider-field-help">一般留空。仅在使用代理或兼容网关时填写；获取访问凭证仍使用 QQ 官方地址。</span>
            </label>
            <button class="bind-button w-full" :disabled="busy === 'qq-config' || !providerConfig.qq.app_id || !providerConfig.qq.app_secret">{{ busy === 'qq-config' ? '开通中…' : '保存并开通 QQ 机器人' }}</button>
          </form>
        </div>

        <div v-else-if="!channel.configured" class="mt-4 sm:mt-5 rounded-2xl border border-amber-500/15 bg-amber-500/[.06] px-4 py-3 text-xs leading-5 text-muted-foreground">
          <strong class="text-foreground">暂未开通</strong>
          <p class="mt-1">该方式尚未由应用管理员开通，创建提醒时会置灰。</p>
        </div>

        <div v-else-if="channel.channel === 'qq' && channel.configured" class="mt-4 sm:mt-5 rounded-2xl border border-rose-500/15 bg-rose-500/[.04] p-4">
          <p class="text-sm font-extrabold">QQ 服务已配置</p>
          <p class="mt-2 text-xs leading-5 text-muted-foreground">QQ 接收人的 OpenID 请在创建提醒时，通过“＋ 添加接收人”填写；这里不再绑定接收人。</p>
          <div class="channel-test mt-3">
            <p class="channel-test-help">发送测试需要填写本次接收者 OpenID，不会保存为提醒接收人。</p>
            <div class="mt-2 flex gap-2">
              <input v-model.trim="testTargets[channel.channel]" class="bind-input min-w-0 flex-1" :type="testInputType(channel.channel)" :placeholder="testPlaceholder(channel.channel)" />
              <button class="channel-button primary shrink-0" :disabled="busy === channel.channel || !testTargets[channel.channel]" @click="test(channel)">{{ busy === channel.channel ? '发送中…' : '发送测试' }}</button>
            </div>
          </div>
        </div>

        <div v-else class="mt-4 sm:mt-5 rounded-2xl border border-amber-500/15 bg-amber-500/[.06] px-4 py-3 text-xs leading-5 text-muted-foreground">
          <strong class="text-foreground">暂未配置</strong>
          <p class="mt-1">该通知方式未配置完成，创建提醒时会置灰，不能选择。</p>
        </div>

        <p v-if="channel.last_error_code" class="mt-3 text-xs text-rose-500">{{ channel.last_error_code }}</p>
      </article>
    </div>

    <Toast :message="toast.message" :type="toast.type" />
    <Teleport to="body"><div v-if="emailSetupOpen" class="fixed inset-0 z-[80] flex items-center justify-center bg-slate-950/35 p-4 backdrop-blur-sm"><form class="w-full max-w-lg rounded-[28px] border border-white/40 bg-surface p-5 shadow-2xl sm:p-8" @submit.prevent="saveSimpleEmail"><div class="flex items-center justify-between gap-4"><div><p class="text-xs font-extrabold uppercase tracking-[.16em] text-violet-500">邮件提醒</p><h2 class="mt-1 text-2xl font-extrabold">邮件通知服务设置</h2></div><button type="button" class="icon-button" aria-label="关闭" @click="emailSetupOpen = false">×</button></div><p class="mt-4 rounded-2xl bg-blue-500/[.07] px-4 py-3 text-xs leading-5 text-blue-700 dark:text-blue-200">这一步设置的是发件邮箱：系统会使用它的 SMTP 服务和授权码发出提醒。设置完成后，请在邮件卡片中再填写提醒要送达的接收邮箱；两个邮箱可以相同，也可以不同。修改配置时请重新填写完整的发件信息，授权码不会回显。</p><label class="modal-label mt-5 sm:mt-7">服务提供商 <span>*</span><select v-model="emailSetup.provider" class="modal-field mt-2"><option value="qq">QQ 邮箱</option><option value="163">163 邮箱</option><option value="gmail">Gmail</option><option value="custom">其他 SMTP 服务</option></select></label><label class="modal-label mt-5">发件邮箱地址 <span>*</span><input v-model.trim="emailSetup.address" class="modal-field mt-2" type="email" maxlength="80" placeholder="请输入发件邮箱地址" /></label><label class="modal-label mt-5">发件人名称 <small>可选</small><input v-model.trim="emailSetup.sender_name" class="modal-field mt-2" maxlength="40" placeholder="例如：我的提醒" /></label><template v-if="emailSetup.provider === 'custom'"><label class="modal-label mt-5">SMTP 服务器 <span>*</span><input v-model.trim="emailSetup.host" class="modal-field mt-2" placeholder="例如 smtp.example.com" /></label><label class="modal-label mt-5">端口 <span>*</span><input v-model.trim="emailSetup.port" class="modal-field mt-2" inputmode="numeric" placeholder="465 或 587" /></label></template><label class="modal-label mt-5">发件邮箱授权码 <span>*</span><input v-model.trim="emailSetup.password" class="modal-field mt-2" type="password" placeholder="请输入发件邮箱对应的授权码（不是邮箱登录密码）" autocomplete="new-password" /></label><p class="mt-4 text-[11px] leading-4 text-muted-foreground">授权码需在对应邮箱服务商的设置中生成，不能填写邮箱登录密码。</p><p class="mt-4 rounded-2xl bg-blue-500/[.07] px-4 py-3 text-xs leading-5 text-blue-700 dark:text-blue-200">{{ emailProviderTip }}</p><div class="mt-7 flex justify-end gap-3"><button type="button" class="channel-button !flex-none" @click="emailSetupOpen = false">取消</button><button class="rounded-xl bg-brand-500 px-5 py-2 text-sm font-bold text-white disabled:opacity-50" :disabled="busy === 'email-config' || !emailSetup.address || !emailSetup.password || (emailSetup.provider === 'custom' && (!emailSetup.host || !emailSetup.port))">{{ busy === 'email-config' ? '保存中…' : (emailSetupEditing ? '保存修改' : '保存并开通') }}</button></div></form></div></Teleport>
    <Teleport to="body"><div v-if="botSetupChannel" class="fixed inset-0 z-[80] flex items-center justify-center bg-slate-950/35 p-4 backdrop-blur-sm"><form class="max-h-[90vh] w-full max-w-lg overflow-y-auto rounded-[28px] border border-white/40 bg-surface p-5 shadow-2xl sm:p-8" @submit.prevent="saveBotCredentials"><div class="flex items-center justify-between gap-4"><div><p class="text-xs font-extrabold uppercase tracking-[.16em] text-violet-500">机器人应用</p><h2 class="mt-1 text-2xl font-extrabold">更换{{ botSetupChannel === 'feishu' ? '飞书' : 'QQ' }}应用凭证</h2></div><button type="button" class="icon-button" aria-label="关闭" @click="botSetupChannel = ''">×</button></div><p class="mt-4 rounded-2xl bg-blue-500/[.07] px-4 py-3 text-xs leading-5 text-blue-700 dark:text-blue-200">保存后，创建提醒时再为对应方式添加接收人：飞书填写工作邮箱、手机号或 OpenID；QQ 填写接收人的 OpenID。</p><template v-if="botSetupChannel === 'feishu'"><label class="modal-label mt-5">App ID <span>*</span><input v-model.trim="feishuConfig.app_id" class="modal-field mt-2" placeholder="例如 cli_xxxxx" autocomplete="off" /></label><label class="modal-label mt-5">App Secret <span>*</span><input v-model.trim="feishuConfig.app_secret" class="modal-field mt-2" type="password" placeholder="请输入新的 App Secret" autocomplete="new-password" /></label></template><template v-else><label class="modal-label mt-5">QQ 机器人 App ID <span>*</span><input v-model.trim="providerConfig.qq.app_id" class="modal-field mt-2" placeholder="例如：1903753507" autocomplete="off" /></label><p class="modal-help">QQ 开放平台创建机器人后生成的唯一标识。</p><label class="modal-label mt-5">QQ 机器人 App Secret <span>*</span><input v-model.trim="providerConfig.qq.app_secret" class="modal-field mt-2" type="password" placeholder="请输入新的 App Secret" autocomplete="new-password" /></label><p class="modal-help">与 App ID 配套的密钥，只用于换取 QQ 访问凭证，请勿分享。</p><label class="modal-label mt-5">机器人主页 / 邀请链接 <small>推荐</small><input v-model.trim="providerConfig.qq.bot_link" class="modal-field mt-2" type="url" placeholder="例如：https://q.qq.com/..." /></label><p class="modal-help">用于显示“打开 QQ 机器人”按钮，不参与消息发送。</p><label class="modal-label mt-5">消息 API 地址 <small>可选</small><input v-model.trim="providerConfig.qq.api_base" class="modal-field mt-2" placeholder="留空使用 https://api.bot.qq.com" /></label><p class="modal-help">一般留空，仅代理或兼容网关场景需要填写；获取访问凭证仍使用 QQ 官方地址。</p></template><div class="mt-7 flex justify-end gap-3"><button type="button" class="channel-button !flex-none" @click="botSetupChannel = ''">取消</button><button class="rounded-xl bg-brand-500 px-5 py-2 text-sm font-bold text-white disabled:opacity-50" :disabled="botSaveDisabled">{{ busy.endsWith('-config') ? '保存中…' : '保存新的凭证' }}</button></div></form></div></Teleport>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useAuthStore } from '../stores/auth'
import Toast from '../components/Toast.vue'
import {
  getChannelStatuses, getNotificationBrand, saveFeishuProvider, saveNotificationBrand, saveProvider, testChannel,
  type ChannelStatus, type ReminderChannel
} from '../api/reminder'

const channels = ref<(ChannelStatus & { last_error_code?: string })[]>([])
const feishuConfig = reactive({ app_id: '', app_secret: '' })
const notificationBrand = ref('提醒事项')
const botSetupChannel = ref<'' | 'feishu' | 'qq'>('')
const editingProvider = ref('')
const emailSetupOpen = ref(false)
const emailSetupEditing = ref(false)
const emailSetup = reactive({ provider: 'qq', address: '', sender_name: '', password: '', host: '', port: '587' })
const providerConfig = reactive({
  email: { host: '', port: '587', from_address: '', username: '', password: '' },
  sms: { webhook_url: '', webhook_token: '' },
  qq: { app_id: '', app_secret: '', bot_link: '', api_base: '' },
})
const testTargets = reactive<Record<string, string>>({})
const busy = ref('')
const loading = ref(true)
const loadError = ref('')
const toast = reactive<{ message: string; type: 'success' | 'error' }>({ message: '', type: 'success' })
const authStore = useAuthStore()
const emailProviderTip = computed(() => ({ qq: 'QQ 邮箱：在“设置 → 账号”开启 POP3/SMTP 或 IMAP/SMTP 服务后生成授权码；系统将自动使用 smtp.qq.com:465。', 163: '163 邮箱：在“设置 → POP3/SMTP/IMAP”开启 SMTP 服务并生成客户端授权密码；系统将自动使用 smtp.163.com:465。', gmail: 'Gmail：请使用应用专用密码；系统将自动使用 smtp.gmail.com:465。', custom: '请向你的邮箱服务商确认 SMTP 服务器、端口和授权码。' } as Record<string, string>)[emailSetup.provider] || '')
const botSaveDisabled = computed(() => botSetupChannel.value === 'feishu'
  ? busy.value === 'feishu-config' || !feishuConfig.app_id || !feishuConfig.app_secret
  : busy.value === 'qq-config' || !providerConfig.qq.app_id || !providerConfig.qq.app_secret)

async function load(quiet = false) {
  if (!quiet) loading.value = true
  try {
    const res = await getChannelStatuses()
    channels.value = res.data.data || []
    loadError.value = ''
  } catch (err: any) {
    // 失败必须显式提示：此前这里没有 catch，渠道卡片会整片空白，用户以为
    // 「渠道不显示」，无法区分真的没有渠道和接口坏了。
    channels.value = []
    if (!quiet) loadError.value = err.response?.data?.message || '读取通知方式失败，请重试'
  } finally { if (!quiet) loading.value = false }
}
async function loadBrand() {
  if (!authStore.isAdmin) return
  try {
    const res = await getNotificationBrand()
    notificationBrand.value = res.data?.data?.name || '提醒事项'
  } catch { /* channel setup stays usable if this optional preference is unavailable */ }
}
async function saveBrand() {
  busy.value = 'notification-brand'
  try {
    const res = await saveNotificationBrand(notificationBrand.value)
    notificationBrand.value = res.data?.data?.name || notificationBrand.value
    show('机器人消息来源名称已保存')
  } catch (err: any) { show(err.response?.data?.message || '保存失败', 'error') }
  finally { busy.value = '' }
}
async function configureFeishu() {
  busy.value = 'feishu-config'
  try {
    await saveFeishuProvider(feishuConfig)
    feishuConfig.app_secret = ''
    await load()
    botSetupChannel.value = ''
    show('飞书机器人已开通，创建提醒时再添加接收人')
  } catch (err: any) { show(err.response?.data?.message || '飞书开通失败', 'error') }
  finally { busy.value = '' }
}
async function configureProvider(provider: 'email' | 'sms' | 'qq') {
  busy.value = `${provider}-config`
  try {
    await saveProvider(provider, providerConfig[provider])
    if (provider === 'email') providerConfig.email.password = ''
    if (provider === 'sms') providerConfig.sms.webhook_token = ''
    if (provider === 'qq') providerConfig.qq.app_secret = ''
    await load()
    editingProvider.value = ''
    if (provider === 'qq') botSetupChannel.value = ''
    show(`${({ email: '邮件', sms: '短信', qq: 'QQ 机器人' } as Record<string, string>)[provider]}配置已保存，创建提醒时再添加接收人`)
    return true
  } catch (err: any) { show(err.response?.data?.message || '开通失败', 'error'); return false }
  finally { busy.value = '' }
}
async function saveBotCredentials() {
  if (botSetupChannel.value === 'feishu') await configureFeishu()
  else if (botSetupChannel.value === 'qq') await configureProvider('qq')
}
function openBotSetup(channel: ReminderChannel) {
  if (channel === 'feishu' || channel === 'qq') botSetupChannel.value = channel
}
function openEmailSetup(editing = false) {
  emailSetupEditing.value = editing
  emailSetupOpen.value = true
}
async function saveSimpleEmail() {
  const presets: Record<string, { host: string; port: string }> = { qq: { host: 'smtp.qq.com', port: '465' }, '163': { host: 'smtp.163.com', port: '465' }, gmail: { host: 'smtp.gmail.com', port: '465' } }
  const server = presets[emailSetup.provider] || { host: emailSetup.host, port: emailSetup.port }
  Object.assign(providerConfig.email, { host: server.host, port: server.port, from_address: emailSetup.address, from_name: emailSetup.sender_name, username: emailSetup.address, password: emailSetup.password })
  if (await configureProvider('email')) emailSetupOpen.value = false
}
async function test(channel: ChannelStatus) {
  const target = (testTargets[channel.channel] || '').trim()
  if (!target) {
    show(`请填写${channel.label}测试接收者`, 'error')
    return
  }
  busy.value = channel.channel
  try {
    await testChannel(channel.channel, target)
    show(`测试${channel.label}已发送`)
  } catch (err: any) { show(err.response?.data?.message || '测试发送失败', 'error') }
  finally { busy.value = '' }
}
function testInputType(channel: ReminderChannel) {
  if (channel === 'email') return 'email'
  if (channel === 'sms') return 'tel'
  return 'text'
}
function testPlaceholder(channel: ReminderChannel) {
  const placeholders: Record<string, string> = {
    email: '接收邮箱，例如 name@example.com',
    sms: '接收手机号，例如 13800138000',
    feishu: '工作邮箱、手机号或 OpenID',
    feishu_webhook: '飞书机器人 Webhook 地址',
    qq: 'QQ 用户 OpenID',
    dingtalk: '钉钉机器人 Webhook 地址',
    dingtalk_webhook: '钉钉机器人 Webhook 地址',
    wecom: 'corpid|corpsecret|agentid|userid',
    wecom_webhook: '企业微信机器人 Webhook 地址',
    pushplus: 'PushPlus Token',
    serverchan: 'Server 酱 SendKey',
    gotify: '服务地址|应用 Token',
    ntfy: '服务地址|Topic，可选 |Token',
    iyuu: 'IYUU Token',
    bafayun: 'UID|Topic',
    bark: 'Bark 服务地址|Device Key，可选 |分组',
  }
  return placeholders[channel] || '请输入本次测试接收者'
}
function show(message: string, type: 'success' | 'error' = 'success') { toast.message = ''; setTimeout(() => Object.assign(toast, { message, type }), 0) }
function statusText(channel: ChannelStatus) {
  return channel.configured ? '已配置' : '未配置'
}
function statusClass(channel: ChannelStatus) {
  return channel.configured ? 'success' : 'warning'
}
function icon(channel: ReminderChannel) {
  const icons: Record<string, string> = {
    inapp: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor"><path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9ZM10 21h4" stroke-width="1.8" stroke-linecap="round"/></svg>',
    email: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor"><rect x="3" y="5" width="18" height="14" rx="3" stroke-width="1.8"/><path d="m4 7 8 6 8-6" stroke-width="1.8" stroke-linejoin="round"/></svg>',
    sms: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor"><path d="M19 4H5a2 2 0 0 0-2 2v10a2 2 0 0 0 2 2h4l3 3 3-3h4a2 2 0 0 0 2-2V6a2 2 0 0 0-2-2Z" stroke-width="1.8"/><path d="M7 9h10M7 13h6" stroke-width="1.8" stroke-linecap="round"/></svg>',
	    feishu: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor"><path d="M7 5.5 12 3l5 2.5v5L12 13 7 10.5v-5ZM7 13.5l5 2.5 5-2.5v5L12 21l-5-2.5v-5Z" stroke-width="1.7" stroke-linejoin="round"/></svg>',
	    qq: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor"><path d="M8 9c0-4 1.8-6 4-6s4 2 4 6c0 1.5.5 3 1.5 4.5.8 1.2 1.2 2.4.5 3.5-.4.6-1 .8-1.8.7-.7 2-2.2 3.3-4.2 3.3s-3.5-1.3-4.2-3.3c-.8.1-1.4-.1-1.8-.7-.7-1.1-.3-2.3.5-3.5C7.5 12 8 10.5 8 9Z" stroke-width="1.7"/></svg>',
	    dingtalk: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor"><path d="M5 5h14v10H9l-4 4V5Z" stroke-width="1.8" stroke-linejoin="round"/><path d="M8 9h8M8 12h5" stroke-width="1.8" stroke-linecap="round"/></svg>',
	    bark: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor"><path d="M12 3 4 6v5c0 5 3.2 8.4 8 10 4.8-1.6 8-5 8-10V6l-8-3Z" stroke-width="1.8" stroke-linejoin="round"/><path d="m9 12 2 2 4-4" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>',
    wecom: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor"><path d="M6 7h12a3 3 0 0 1 3 3v5a3 3 0 0 1-3 3h-5l-3 3v-3H6a3 3 0 0 1-3-3v-5a3 3 0 0 1 3-3Z" stroke-width="1.8" stroke-linejoin="round"/><path d="M8 12h8" stroke-width="1.8" stroke-linecap="round"/></svg>',
  }
  return icons[channel] || icons.inapp
}
onMounted(async () => { await load(); await loadBrand() })
</script>

<style scoped>
.channel-card { @apply rounded-[24px] border border-border/70 bg-surface/85 p-4 shadow-[0_16px_44px_-34px_rgba(15,23,42,.55)] backdrop-blur-xl transition hover:-translate-y-0.5 hover:border-brand-500/15 sm:p-5; }
.channel-icon { @apply flex h-10 w-10 shrink-0 items-center justify-center rounded-xl sm:h-12 sm:w-12 sm:rounded-2xl; }
.channel-icon :deep(svg) { @apply h-5 w-5 sm:h-6 sm:w-6; }
.channel-icon.inapp { @apply bg-blue-500/10 text-blue-500; }.channel-icon.email { @apply bg-violet-500/10 text-violet-500; }.channel-icon.sms { @apply bg-amber-500/10 text-amber-500; }.channel-icon.feishu,.channel-icon.feishu_webhook { @apply bg-cyan-500/10 text-cyan-500; }.channel-icon.qq { @apply bg-rose-500/10 text-rose-500; }.channel-icon.dingtalk,.channel-icon.dingtalk_webhook { @apply bg-sky-500/10 text-sky-500; }.channel-icon.wecom,.channel-icon.wecom_webhook { @apply bg-emerald-500/10 text-emerald-500; }.channel-icon.bark { @apply bg-orange-500/10 text-orange-500; }.channel-icon.pushplus,.channel-icon.serverchan,.channel-icon.gotify,.channel-icon.ntfy,.channel-icon.iyuu,.channel-icon.bafayun { @apply bg-slate-500/10 text-slate-500; }
.status-pill { @apply rounded-full px-2 py-0.5 text-[9px] font-extrabold; }.status-pill.success { @apply bg-emerald-500/10 text-emerald-500; }.status-pill.warning { @apply bg-amber-500/10 text-amber-500; }.status-pill.muted { @apply bg-muted text-muted-foreground; }
.toggle { @apply relative h-7 w-12 shrink-0 rounded-full bg-muted transition; }.toggle span { @apply absolute left-1 top-1 h-5 w-5 rounded-full bg-white shadow transition; }.toggle.on { @apply bg-emerald-500; }.toggle.on span { transform: translateX(20px); }
.bind-input { @apply h-10 min-w-0 flex-1 rounded-xl border border-border bg-surface px-3.5 text-sm outline-none focus:border-brand-500/40 focus:ring-4 focus:ring-brand-500/10 sm:h-11 sm:rounded-2xl; }
.bind-button { @apply h-10 rounded-xl bg-brand-500 px-4 text-sm font-bold text-white disabled:opacity-50 sm:h-11 sm:rounded-2xl; }
.channel-button { @apply flex-1 rounded-xl border border-border bg-surface px-3 py-2 text-xs font-bold text-foreground transition hover:bg-muted disabled:opacity-45; }.channel-button.primary { @apply border-brand-500/20 bg-brand-500/10 text-brand-600 dark:text-brand-300; }.channel-button.danger { @apply text-rose-500; }
.channel-test { @apply rounded-2xl border border-brand-500/15 bg-brand-500/[.04] p-3; }.channel-test-help { @apply text-[10px] leading-4 text-muted-foreground; }
.provider-form { @apply mt-4 rounded-2xl border border-violet-500/15 bg-violet-500/[.05] p-4 sm:mt-5; }.provider-help { @apply mt-1 text-xs leading-5 text-muted-foreground; }
.provider-field { @apply block; }.provider-field-label { @apply flex items-center gap-2 text-xs font-bold text-foreground; }.provider-badge { @apply rounded-full px-1.5 py-0.5 text-[9px] font-bold; }.provider-badge.required { @apply bg-rose-500/10 text-rose-500; }.provider-badge.recommended { @apply bg-emerald-500/10 text-emerald-600 dark:text-emerald-300; }.provider-badge.optional { @apply bg-muted text-muted-foreground; }.provider-field-help { @apply mt-1 text-[11px] leading-4 text-muted-foreground; }
.modal-label { @apply block text-sm font-bold text-foreground; }.modal-label span { @apply text-rose-500; }.modal-label small { @apply ml-1 text-xs font-medium text-muted-foreground; }.modal-field { @apply h-11 w-full rounded-xl border border-border bg-surface px-3.5 text-sm font-normal outline-none focus:border-brand-500 focus:ring-4 focus:ring-brand-500/10 sm:h-12 sm:rounded-2xl sm:px-4; }
.modal-help { @apply mt-1 text-[11px] leading-4 text-muted-foreground; }
</style>
