package reminder

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"smallgo/server/database"

	"github.com/gin-gonic/gin"
)

func TestSMTPErrorMessageExplainsQQAuthorizationCode(t *testing.T) {
	got := smtpErrorMessage("smtp.qq.com", errors.New(`535 "Login fail. Account is abnormal, service is not open, password is incorrect"`))

	for _, want := range []string{"QQ 邮箱 SMTP 登录失败", "开启 POP3/SMTP 或 IMAP/SMTP 服务", "新生成的授权码", "不是 QQ/邮箱登录密码"} {
		if !strings.Contains(got, want) {
			t.Fatalf("smtpErrorMessage() = %q, want it to contain %q", got, want)
		}
	}
	if strings.Contains(got, "Account is abnormal") {
		t.Fatalf("smtpErrorMessage() exposed the provider's ambiguous raw error: %q", got)
	}
}

func TestSMTPErrorMessageKeepsNonAuthenticationFailure(t *testing.T) {
	got := smtpErrorMessage("smtp.example.com", errors.New("dial tcp: connection refused"))
	if got != "邮件发送失败：dial tcp: connection refused" {
		t.Fatalf("smtpErrorMessage() = %q", got)
	}
}

func TestNotificationContentOmitsScheduledTime(t *testing.T) {
	dueAt := time.Date(2026, time.September, 30, 8, 30, 0, 0, time.Local)
	got := notificationContent(Reminder{Title: "服用维生素", DueAt: &dueAt, Notes: "AD、维A、铁、钙"})
	if strings.Contains(got, "计划时间") {
		t.Fatalf("notification content still includes scheduled time: %q", got)
	}
	if got != "你有一条新的提醒\n\nAD、维A、铁、钙" {
		t.Fatalf("notification content = %q", got)
	}
}

func TestNotificationContentUsesSelectedTemplate(t *testing.T) {
	got := notificationContent(Reminder{
		Title:                "服用维生素",
		NotificationTemplate: "warm",
		Notes:                "AD、维A、铁、钙",
	})
	if got != "温馨提醒：请记得完成「服用维生素」\n\nAD、维A、铁、钙" {
		t.Fatalf("notification content = %q", got)
	}
}

func TestResolveFeishuOpenIDByEmail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			_, _ = w.Write([]byte(`{"code":0,"tenant_access_token":"tenant-token"}`))
		case "/open-apis/contact/v3/users/batch_get_id":
			if got := r.URL.Query().Get("user_id_type"); got != "open_id" {
				t.Fatalf("user_id_type = %q", got)
			}
			var body map[string][]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if len(body["emails"]) != 1 || body["emails"][0] != "person@example.com" {
				t.Fatalf("unexpected lookup payload: %#v", body)
			}
			_, _ = w.Write([]byte(`{"code":0,"data":{"user_list":[{"user_id":"ou_test_user"}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	t.Setenv("FEISHU_APP_ID", "cli_test")
	t.Setenv("FEISHU_APP_SECRET", "secret")
	t.Setenv("FEISHU_BASE_URL", server.URL)

	got, err := resolveFeishuOpenID(context.Background(), appDB, "person@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if got != "ou_test_user" {
		t.Fatalf("OpenID = %q", got)
	}
}

func TestNormalizeDingTalkWebhook(t *testing.T) {
	got, err := normalizeTarget(ChannelDingTalk, "https://oapi.dingtalk.com/robot/send?access_token=test-token")
	if err != nil {
		t.Fatal(err)
	}
	if want := "https://oapi.dingtalk.com/robot/send?access_token=test-token"; got != want {
		t.Fatalf("webhook = %q, want %q", got, want)
	}
	for _, raw := range []string{
		"http://oapi.dingtalk.com/robot/send?access_token=test-token",
		"https://example.com/robot/send?access_token=test-token",
		"https://oapi.dingtalk.com/robot/send?access_token=test-token&sign=stale",
		"https://oapi.dingtalk.com/robot/send",
	} {
		if _, err := normalizeTarget(ChannelDingTalk, raw); err == nil {
			t.Fatalf("expected %q to be rejected", raw)
		}
	}
	if got := maskTarget(ChannelDingTalk, "https://oapi.dingtalk.com/robot/send?access_token=secret"); got != "钉钉机器人 Webhook（已加密）" {
		t.Fatalf("masked webhook = %q", got)
	}
}

func TestHandleTestChannelRequiresTargetAndReminderTitle(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	t.Run("target is required", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(recorder)
		context.Request = httptest.NewRequest(http.MethodPost, "/api/reminder/channels/email/test", strings.NewReader(`{}`))
		context.Params = gin.Params{{Key: "channel", Value: ChannelEmail}}
		context.Set("userID", uint(7))
		handleTestChannel(appDB)(context)

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected missing target to return 400, got %d: %s", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("current reminder title is required", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(recorder)
		context.Request = httptest.NewRequest(http.MethodPost, "/api/reminder/channels/email/test", strings.NewReader(`{"target":"person@example.com","reminder":{"notes":"备注"}}`))
		context.Params = gin.Params{{Key: "channel", Value: ChannelEmail}}
		context.Set("userID", uint(7))
		handleTestChannel(appDB)(context)

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected missing title to return 400, got %d: %s", recorder.Code, recorder.Body.String())
		}
		if !strings.Contains(recorder.Body.String(), "请输入提醒标题") {
			t.Fatalf("expected title validation message, got %s", recorder.Body.String())
		}
	})
}

func TestSendDingTalkUsesKeywordCompatibleText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		var payload struct {
			MsgType string `json:"msgtype"`
			Text    struct {
				Content string `json:"content"`
			} `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.MsgType != "text" || !strings.Contains(payload.Text.Content, "提醒：喝水") {
			t.Fatalf("unexpected DingTalk payload: %#v", payload)
		}
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	defer server.Close()

	if _, err := sendDingTalk(context.Background(), nil, server.URL, Reminder{Title: "喝水"}); err != nil {
		t.Fatal(err)
	}
}

func createEmailBinding(t *testing.T, userID uint, target, status string) ChannelBinding {
	t.Helper()
	t.Setenv("REMINDER_DATA_KEY", "test-key")
	encrypted, err := encryptTarget(appDB, target)
	if err != nil {
		t.Fatal(err)
	}
	binding := ChannelBinding{
		UserID: userID, Channel: ChannelEmail, Target: encrypted,
		TargetMasked: maskTarget(ChannelEmail, target), Status: status,
	}
	if err := appDB.Create(&binding).Error; err != nil {
		t.Fatal(err)
	}
	return binding
}

func TestUpgradeDropsLegacySingleBindingUniqueIndex(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	// Recreate the constraint shipped before multiple email bindings.
	if err := appDB.Exec("CREATE UNIQUE INDEX idx_user_channel_binding ON channel_bindings (user_id, channel)").Error; err != nil {
		t.Fatal(err)
	}
	if err := database.RunUpgrades(appDB, "v0.3.0", database.Upgrades); err != nil {
		t.Fatal(err)
	}
	createEmailBinding(t, 7, "one@qq.com", "active")
	// This insert would violate the legacy unique index if it survived.
	createEmailBinding(t, 7, "two@163.com", "active")
}

func TestActiveEmailRecipientsReturnsAllActiveBindings(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	const userID = 7
	createEmailBinding(t, userID, "one@qq.com", "active")
	createEmailBinding(t, userID, "two@163.com", "active")
	createEmailBinding(t, userID, "disabled@gmail.com", "disabled")
	createEmailBinding(t, 8, "other@qq.com", "active")

	recipients, err := activeEmailRecipients(appDB, userID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(recipients) != 2 {
		t.Fatalf("expected 2 active recipients, got %v", recipients)
	}
	found := map[string]bool{}
	for _, address := range recipients {
		found[strings.ToLower(address)] = true
	}
	if !found["one@qq.com"] || !found["two@163.com"] {
		t.Fatalf("missing expected recipients: %v", recipients)
	}
}

func TestEmailTargetExistsIgnoresCaseAndOtherUsers(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	createEmailBinding(t, 7, "one@qq.com", "active")

	duplicate, err := emailTargetExists(appDB, 7, "ONE@qq.com")
	if err != nil {
		t.Fatal(err)
	}
	if !duplicate {
		t.Fatal("expected case-insensitive duplicate detection")
	}
	duplicate, err = emailTargetExists(appDB, 7, "two@qq.com")
	if err != nil {
		t.Fatal(err)
	}
	if duplicate {
		t.Fatal("different address must not be flagged as duplicate")
	}
	duplicate, err = emailTargetExists(appDB, 8, "one@qq.com")
	if err != nil {
		t.Fatal(err)
	}
	if duplicate {
		t.Fatal("other user's binding must not block this user")
	}
}

func TestChannelStatusesExposeEmailBindingList(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	const userID = 7
	first := createEmailBinding(t, userID, "one@qq.com", "active")
	second := createEmailBinding(t, userID, "two@163.com", "disabled")

	for _, status := range channelStatuses(appDB, userID) {
		if status.Channel != ChannelEmail {
			continue
		}
		if !status.Bound {
			t.Fatal("email channel should be reported as bound")
		}
		if status.Status != "active" {
			t.Fatalf("expected overall active status, got %q", status.Status)
		}
		if len(status.Bindings) != 2 {
			t.Fatalf("expected 2 binding items, got %v", status.Bindings)
		}
		ids := map[uint]struct{ target, masked string }{
			first.ID:  {target: "one@qq.com", masked: first.TargetMasked},
			second.ID: {target: "two@163.com", masked: second.TargetMasked},
		}
		for _, item := range status.Bindings {
			want, ok := ids[item.ID]
			if !ok || want.target != item.Target || want.masked != item.TargetMasked {
				t.Fatalf("unexpected binding item: %+v", item)
			}
		}
		return
	}
	t.Fatal("email channel status not found")
}

func TestHandleDeleteChannelBindingRemovesSingleEmail(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	const userID = 7
	kept := createEmailBinding(t, userID, "one@qq.com", "active")
	removed := createEmailBinding(t, userID, "two@163.com", "active")
	other := createEmailBinding(t, 8, "other@qq.com", "active")

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodDelete, "/api/reminder/channels/email/bindings/"+strconv.FormatUint(uint64(removed.ID), 10), nil)
	context.Params = gin.Params{{Key: "channel", Value: ChannelEmail}, {Key: "id", Value: strconv.FormatUint(uint64(removed.ID), 10)}}
	context.Set("userID", userID)
	handleDeleteChannelBinding(appDB)(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var count int64
	if err := appDB.Model(&ChannelBinding{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 remaining binding for user, got %d", count)
	}
	var remaining ChannelBinding
	if err := appDB.First(&remaining, kept.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := appDB.First(&ChannelBinding{}, other.ID).Error; err != nil {
		t.Fatal("other user's binding must stay")
	}
}

func TestActiveEmailRecipientsFiltersByReminderTargets(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	const userID = 7
	first := createEmailBinding(t, userID, "one@qq.com", "active")
	createEmailBinding(t, userID, "two@163.com", "active")

	recipients, err := activeEmailRecipients(appDB, userID, []uint{first.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(recipients) != 1 || !strings.EqualFold(recipients[0], "one@qq.com") {
		t.Fatalf("expected only the selected mailbox, got %v", recipients)
	}

	// A selection that matches nothing (mailbox unbound meanwhile) stays empty;
	// the delivery layer reports the missing target instead of redirecting.
	recipients, err = activeEmailRecipients(appDB, userID, []uint{9999})
	if err != nil {
		t.Fatal(err)
	}
	if len(recipients) != 0 {
		t.Fatalf("expected no recipients for a missing selection, got %v", recipients)
	}
}

func TestReminderTargetsRoundTripAndOwnership(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	const userID = 7
	owned := createEmailBinding(t, userID, "one@qq.com", "active")
	createEmailBinding(t, 8, "other@qq.com", "active")

	due := time.Now().Add(2 * time.Hour).Truncate(time.Second)
	_, err := createReminder(appDB, userID, SaveReminderInput{
		Title: "只发工作邮箱", DueAt: &due, RepeatRule: "none",
		Channels:       []string{ChannelInApp, ChannelEmail},
		ChannelTargets: map[string][]uint{ChannelEmail: {owned.ID, 8, 9999}},
	})
	if err == nil || !strings.Contains(err.Error(), "不存在或已停用") {
		t.Fatalf("expected invalid target rejection, got %v", err)
	}
	created, err := createReminder(appDB, userID, SaveReminderInput{
		Title: "只发工作邮箱", DueAt: &due, RepeatRule: "none",
		Channels:       []string{ChannelInApp, ChannelEmail},
		ChannelTargets: map[string][]uint{ChannelEmail: {owned.ID}},
	})
	if err != nil {
		t.Fatal(err)
	}
	targets := created.ChannelTargets[ChannelEmail]
	if len(targets) != 1 || targets[0] != owned.ID {
		t.Fatalf("expected only the owned binding id to survive, got %v", targets)
	}

	var link ReminderChannel
	if err := appDB.Where("reminder_id = ? AND channel = ?", created.ID, ChannelEmail).First(&link).Error; err != nil {
		t.Fatal(err)
	}
	if stored := parseChannelTargets(link.Targets); len(stored) != 1 || stored[0] != owned.ID {
		t.Fatalf("unexpected stored targets: %v", parseChannelTargets(link.Targets))
	}

	// Clearing the selection is rejected because every external channel now
	// needs at least one explicit receiving target.
	_, err = updateReminder(appDB, userID, created.ID, SaveReminderInput{
		Title: "只发工作邮箱", DueAt: &due, RepeatRule: "none",
		Channels:       []string{ChannelInApp, ChannelEmail},
		ChannelTargets: map[string][]uint{},
		Version:        created.Version,
	})
	if err == nil || !strings.Contains(err.Error(), "至少选择一个接收人") {
		t.Fatalf("expected empty target rejection, got %v", err)
	}
}
