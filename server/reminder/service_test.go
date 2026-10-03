package reminder

import (
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"smallgo/server/database"

	"gorm.io/gorm"
)

func testDB(t *testing.T) func() {
	t.Helper()
	db, err := database.InitDB(filepath.Join(t.TempDir(), "reminder-test.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	appDB = db
	return func() { _ = database.CloseDB(db) }
}

func TestCreateAndUpdateReminderRebuildsJobIdempotently(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	email := createEmailBinding(t, 42, "test@example.com", "active")
	due := time.Now().Add(2 * time.Hour).Truncate(time.Second)
	created, err := createReminder(appDB, 42, SaveReminderInput{
		Title: "测试提醒", DueAt: &due, RepeatRule: "none",
		Channels: []string{ChannelInApp, ChannelEmail}, ChannelTargets: map[string][]uint{ChannelEmail: {email.ID}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(created.Channels) != 2 {
		t.Fatalf("expected 2 channels, got %v", created.Channels)
	}

	created.Notes = "更新备注但不改变时间"
	updated, err := updateReminder(appDB, 42, created.ID, SaveReminderInput{
		Title: created.Title, Notes: created.Notes, ListID: created.ListID,
		DueAt: &due, RepeatRule: "none", Channels: created.Channels,
		ChannelTargets: map[string][]uint{ChannelEmail: {email.ID}}, Version: created.Version,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Version != created.Version+1 {
		t.Fatalf("expected version %d, got %d", created.Version+1, updated.Version)
	}

	var activeJobs int64
	if err := appDB.Model(&DeliveryJob{}).
		Where("reminder_id = ? AND status = ?", created.ID, "pending").
		Count(&activeJobs).Error; err != nil {
		t.Fatal(err)
	}
	if activeJobs != 2 {
		t.Fatalf("expected 2 active jobs, got %d", activeJobs)
	}
}

func TestCompleteRecurringReminderAdvancesFromScheduledTime(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	loc := shanghai()
	due := time.Date(2026, time.January, 31, 9, 0, 0, 0, loc)
	created, err := createReminder(appDB, 7, SaveReminderInput{
		Title: "月末提醒", DueAt: &due, RepeatRule: "monthly", Channels: []string{ChannelInApp},
	})
	if err != nil {
		t.Fatal(err)
	}
	next, err := completeReminder(appDB, 7, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, time.February, 28, 9, 0, 0, 0, loc)
	if next.DueAt == nil || !next.DueAt.Equal(want) {
		t.Fatalf("expected %v, got %v", want, next.DueAt)
	}
	if next.CompletedAt != nil {
		t.Fatal("recurring reminder should remain active")
	}

	var histories int64
	if err := appDB.Model(&Completion{}).Where("reminder_id = ?", created.ID).Count(&histories).Error; err != nil {
		t.Fatal(err)
	}
	if histories != 1 {
		t.Fatalf("expected completion history, got %d", histories)
	}
}

func TestRecurringReminderStopsAfterEndAt(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	loc := shanghai()
	start := time.Date(2026, time.January, 1, 9, 0, 0, 0, loc)
	end := time.Date(2026, time.January, 2, 9, 0, 0, 0, loc)
	created, err := createReminder(appDB, 8, SaveReminderInput{
		Title: "循环截止", DueAt: &start, EndAt: &end, RepeatRule: "daily", Channels: []string{ChannelInApp},
	})
	if err != nil {
		t.Fatal(err)
	}

	next, err := completeReminder(appDB, 8, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if next.CompletedAt != nil || next.DueAt == nil || !next.DueAt.Equal(end) {
		t.Fatalf("expected the end occurrence to remain active, got due=%v completed=%v", next.DueAt, next.CompletedAt)
	}

	last, err := completeReminder(appDB, 8, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if last.CompletedAt == nil {
		t.Fatal("expected the recurring reminder to complete after its end time")
	}
	var pending int64
	if err := appDB.Model(&DeliveryJob{}).Where("reminder_id = ? AND status IN ?", created.ID, []string{"pending", "retry"}).Count(&pending).Error; err != nil {
		t.Fatal(err)
	}
	if pending != 0 {
		t.Fatalf("expected no pending jobs after the final occurrence, got %d", pending)
	}
}

func TestOneTimeReminderIgnoresEndAt(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	start := time.Now().Add(time.Hour).Truncate(time.Second)
	end := start.Add(time.Hour)
	created, err := createReminder(appDB, 9, SaveReminderInput{
		Title: "一次性提醒", DueAt: &start, EndAt: &end, RepeatRule: "none", Channels: []string{ChannelInApp},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.EndAt != nil {
		t.Fatalf("one-time reminder must not keep an end time, got %v", created.EndAt)
	}
}

func TestCronReminderAdvancesByExpression(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	loc := shanghai()
	start := time.Date(2026, time.January, 1, 8, 30, 0, 0, loc)
	created, err := createReminder(appDB, 10, SaveReminderInput{
		Title: "Cron 吃药提醒", DueAt: &start, RepeatRule: "cron",
		CronExpr: "30 8-20/2 * * *", Channels: []string{ChannelInApp},
	})
	if err != nil {
		t.Fatal(err)
	}
	next, err := completeReminder(appDB, 10, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, time.January, 1, 10, 30, 0, 0, loc)
	if next.DueAt == nil || !next.DueAt.Equal(want) {
		t.Fatalf("expected next Cron occurrence %v, got %v", want, next.DueAt)
	}
}

func TestInvalidCronReminderRejected(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	start := time.Now().Add(time.Hour).Truncate(time.Second)
	_, err := createReminder(appDB, 11, SaveReminderInput{
		Title: "无效 Cron", DueAt: &start, RepeatRule: "cron",
		CronExpr: "not-a-cron", Channels: []string{ChannelInApp},
	})
	if err == nil {
		t.Fatal("expected invalid Cron expression to be rejected")
	}
}

func TestIntervalCronStaysInsideTimeWindow(t *testing.T) {
	loc := shanghai()
	start := time.Date(2026, time.January, 1, 12, 0, 0, 0, loc)
	expr := "@interval/v1|step=1800|start=43200|end=46800|base=0 0 12 * * *"
	first, err := nextCronOccurrence(start, expr)
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, time.January, 1, 12, 30, 0, 0, loc); !first.Equal(want) {
		t.Fatalf("expected first interval occurrence %v, got %v", want, first)
	}
	second, err := nextCronOccurrence(first, expr)
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, time.January, 1, 13, 0, 0, 0, loc); !second.Equal(want) {
		t.Fatalf("expected end-boundary occurrence %v, got %v", want, second)
	}
	third, err := nextCronOccurrence(second, expr)
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, time.January, 2, 12, 0, 0, 0, loc); !third.Equal(want) {
		t.Fatalf("expected next-day start occurrence %v, got %v", want, third)
	}
}

func TestIntervalCronUsesWindowStartBeforeFirstOccurrence(t *testing.T) {
	loc := shanghai()
	before := time.Date(2026, time.January, 1, 11, 45, 0, 0, loc)
	expr := "@interval/v1|step=1800|start=43200|end=46800|base=0 0 12 * * *"
	next, err := nextCronOccurrence(before, expr)
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, time.January, 1, 12, 0, 0, 0, loc); !next.Equal(want) {
		t.Fatalf("expected window start %v, got %v", want, next)
	}
}

func TestIntervalCronRejectsWindowLength(t *testing.T) {
	_, err := parseCronSchedule("@interval/v1|step=3600|start=43200|end=46800|base=0 0 12 * * *")
	if err == nil {
		t.Fatal("expected an interval equal to the window length to be rejected")
	}
}

func TestYearlyLeapDayUsesEndOfFebruary(t *testing.T) {
	loc := shanghai()
	start := time.Date(2024, time.February, 29, 8, 30, 0, 0, loc)
	got := nextOccurrence(start, "yearly")
	want := time.Date(2025, time.February, 28, 8, 30, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestInAppDeliveryCreatesNotificationOnce(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	events, unsubscribe := reminderRealtime.subscribe(9, "test-client")
	defer unsubscribe()

	due := time.Now().Add(-time.Minute).Truncate(time.Second)
	created, err := createReminder(appDB, 9, SaveReminderInput{
		Title: "已经到期", DueAt: &due, RepeatRule: "none", Channels: []string{ChannelInApp},
	})
	if err != nil {
		t.Fatal(err)
	}

	dispatchDueJobs(appDB)
	dispatchDueJobs(appDB)

	var notifications int64
	if err := appDB.Model(&Notification{}).Where("user_id = ? AND reminder_id = ?", 9, created.ID).Count(&notifications).Error; err != nil {
		t.Fatal(err)
	}
	if notifications != 1 {
		t.Fatalf("expected one notification, got %d", notifications)
	}
	var job DeliveryJob
	if err := appDB.Where("reminder_id = ? AND channel = ?", created.ID, ChannelInApp).First(&job).Error; err != nil {
		t.Fatal(err)
	}
	if job.Status != "succeeded" || job.AttemptCount != 1 {
		t.Fatalf("unexpected job state: status=%s attempts=%d", job.Status, job.AttemptCount)
	}
	select {
	case event := <-events:
		if event.Type != "notification.created" || event.Notification == nil || event.Notification.ReminderID == nil || *event.Notification.ReminderID != created.ID {
			t.Fatalf("unexpected realtime event: %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("expected a realtime notification event")
	}
}

func TestHandlingReminderRetiresUnreadDueNotification(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	// 三条已到点的提醒各发一条站内通知，分别用完成、稍后提醒、删除来处理；
	// 无论从哪条路处理，未读的到点通知都必须标为已读——跨设备的到点补弹
	// 靠已读状态判断该不该弹，否则会把已处理掉的提醒再弹出来。
	due := time.Now().Add(-time.Minute).Truncate(time.Second)
	ids := make([]uint, 0, 3)
	for _, title := range []string{"完成路径", "稍后路径", "删除路径"} {
		created, err := createReminder(appDB, 7, SaveReminderInput{
			Title: title, DueAt: &due, RepeatRule: "none", Channels: []string{ChannelInApp},
		})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, created.ID)
	}
	dispatchDueJobs(appDB)

	unreadFor := func(reminderID uint) int64 {
		t.Helper()
		var unread int64
		if err := appDB.Model(&Notification{}).
			Where("user_id = ? AND reminder_id = ? AND type = ? AND read_at IS NULL", 7, reminderID, "reminder_due").
			Count(&unread).Error; err != nil {
			t.Fatal(err)
		}
		return unread
	}
	for _, id := range ids {
		if unreadFor(id) != 1 {
			t.Fatalf("reminder %d: expected one unread due notification before handling", id)
		}
	}

	if _, err := completeReminder(appDB, 7, ids[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := snoozeReminder(appDB, 7, ids[1], time.Now().Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := deleteReminder(appDB, 7, ids[2]); err != nil {
		t.Fatal(err)
	}

	for i, id := range ids {
		if unreadFor(id) != 0 {
			t.Fatalf("reminder %d (case %d): due notification still unread after handling", id, i)
		}
	}
}

func TestReminderOwnershipIsolation(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	created, err := createReminder(appDB, 100, SaveReminderInput{
		Title: "用户 A 的提醒", RepeatRule: "none", Channels: []string{ChannelInApp},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := getReminder(appDB, 200, created.ID); err == nil {
		t.Fatal("another user must not read the reminder")
	}
	if err := deleteReminder(appDB, 200, created.ID); err == nil {
		t.Fatal("another user must not delete the reminder")
	}
}

func TestListRemindersLoadsRelatedDataInConstantQueries(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	const userID = 55
	email := createEmailBinding(t, userID, "batch@example.com", "active")
	for i := 0; i < 30; i++ {
		_, err := createReminder(appDB, userID, SaveReminderInput{
			Title: "批量提醒", RepeatRule: "none", Channels: []string{ChannelInApp, ChannelEmail},
			ChannelTargets: map[string][]uint{ChannelEmail: {email.ID}},
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	var queryCount atomic.Int64
	if err := appDB.Callback().Query().Before("gorm:query").Register("test:list-query-count", func(*gorm.DB) {
		queryCount.Add(1)
	}); err != nil {
		t.Fatal(err)
	}

	items, err := listReminders(appDB, userID, "all", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 30 {
		t.Fatalf("expected 30 reminders, got %d", len(items))
	}
	if got := queryCount.Load(); got != 3 {
		t.Fatalf("expected 3 fixed queries, got %d", got)
	}
	for _, item := range items {
		if item.ListName == "" || len(item.Channels) != 2 {
			t.Fatalf("missing related data: list=%q channels=%v", item.ListName, item.Channels)
		}
	}
}

func TestRepeatNotifyRequeuesUntilCompleted(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	due := time.Now().Add(-time.Minute).Truncate(time.Second)
	created, err := createReminder(appDB, 9, SaveReminderInput{
		Title: "每小时催一次", DueAt: &due, RepeatRule: "none",
		RepeatNotifyMinutes: 30, Channels: []string{ChannelInApp},
	})
	if err != nil {
		t.Fatal(err)
	}

	dispatchDueJobs(appDB)

	var notifications int64
	if err := appDB.Model(&Notification{}).Where("user_id = ? AND reminder_id = ?", 9, created.ID).Count(&notifications).Error; err != nil {
		t.Fatal(err)
	}
	if notifications != 1 {
		t.Fatalf("expected one notification after the first due time, got %d", notifications)
	}
	var chain DeliveryJob
	if err := appDB.Where("reminder_id = ? AND channel = ? AND status = ?", created.ID, ChannelInApp, "pending").First(&chain).Error; err != nil {
		t.Fatal("expected a follow-up repeat-notify job")
	}
	if want := due.Add(30 * time.Minute).UTC(); !chain.ScheduledFor.Equal(want) {
		t.Fatalf("expected chain job at %s, got %s", want, chain.ScheduledFor)
	}

	// Completing the reminder cancels the pending follow-up, so no further
	// notifications are produced.
	if _, err := completeReminder(appDB, 9, created.ID); err != nil {
		t.Fatal(err)
	}
	dispatchDueJobs(appDB)

	if err := appDB.Model(&Notification{}).Where("user_id = ? AND reminder_id = ?", 9, created.ID).Count(&notifications).Error; err != nil {
		t.Fatal(err)
	}
	if notifications != 1 {
		t.Fatalf("expected no extra notification after completion, got %d", notifications)
	}
	if err := appDB.First(&chain, chain.ID).Error; err != nil {
		t.Fatal(err)
	}
	if chain.Status != "cancelled" {
		t.Fatalf("expected chain job cancelled after completion, got %s", chain.Status)
	}
}

func TestRepeatNotifyRecurringAdvancesOnCompletion(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	due := time.Now().Add(-time.Minute).Truncate(time.Second)
	created, err := createReminder(appDB, 9, SaveReminderInput{
		Title: "每天下班提醒", DueAt: &due, RepeatRule: "daily",
		RepeatNotifyMinutes: 60, Channels: []string{ChannelInApp},
	})
	if err != nil {
		t.Fatal(err)
	}

	dispatchDueJobs(appDB)

	var chain DeliveryJob
	if err := appDB.Where("reminder_id = ? AND channel = ? AND status = ?", created.ID, ChannelInApp, "pending").First(&chain).Error; err != nil {
		t.Fatal("expected repeat-notify chain job")
	}
	if want := due.Add(time.Hour).UTC(); !chain.ScheduledFor.Equal(want) {
		t.Fatalf("expected chain job at %s, got %s", want, chain.ScheduledFor)
	}

	// Completing advances the daily rule to tomorrow and drops the pending
	// hourly chain in favour of the next day's occurrence.
	updated, err := completeReminder(appDB, 9, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.DueAt == nil || !updated.DueAt.After(due.Add(23*time.Hour)) {
		t.Fatalf("expected daily recurrence to advance about a day, got %v", updated.DueAt)
	}
	var jobs []DeliveryJob
	if err := appDB.Where("reminder_id = ?", created.ID).Find(&jobs).Error; err != nil {
		t.Fatal(err)
	}
	pending := 0
	for _, job := range jobs {
		if job.Status == "pending" {
			pending++
		}
	}
	if pending != 1 {
		t.Fatalf("expected exactly one pending job after completion, got %d", pending)
	}
}

// 回归：链条任务必须真的能派发响铃。此前派发时的同瞬间校验会把
// ScheduledFor≠DueAt 的链条任务当陈旧任务取消，导致过期重复提醒只响第一次。
func TestRepeatNotifyChainKeepsFiringWhileUncompleted(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	due := time.Now().Add(-31 * time.Minute).Truncate(time.Second)
	created, err := createReminder(appDB, 10, SaveReminderInput{
		Title: "每半小时催一次", DueAt: &due, RepeatRule: "none",
		RepeatNotifyMinutes: 30, Channels: []string{ChannelInApp},
	})
	if err != nil {
		t.Fatal(err)
	}

	countNotifications := func() int64 {
		var n int64
		if err := appDB.Model(&Notification{}).Where("user_id = ? AND reminder_id = ?", 10, created.ID).Count(&n).Error; err != nil {
			t.Fatal(err)
		}
		return n
	}

	// 第一次派发：到期通知 + 排出 due+30m 的链条任务（此刻已到期）。
	dispatchDueJobs(appDB)
	if got := countNotifications(); got != 1 {
		t.Fatalf("expected 1 notification after first dispatch, got %d", got)
	}

	// 把链条任务拨到“已到期但晚于 DueAt”（模拟时间流逝到下一个间隔），
	// 再次派发必须正常响铃而不是被同瞬间校验当陈旧任务取消。
	var chain DeliveryJob
	if err := appDB.Where("reminder_id = ? AND channel = ? AND status = ?", created.ID, ChannelInApp, "pending").First(&chain).Error; err != nil {
		t.Fatal("expected a follow-up repeat-notify job")
	}
	fired := time.Now().UTC().Add(-time.Minute)
	if err := appDB.Model(&DeliveryJob{}).Where("id = ?", chain.ID).Updates(map[string]interface{}{
		"scheduled_for": fired, "run_at": fired,
	}).Error; err != nil {
		t.Fatal(err)
	}
	dispatchDueJobs(appDB)
	if got := countNotifications(); got != 2 {
		t.Fatalf("expected repeat-notify chain to fire a 2nd notification, got %d", got)
	}
	var followUp DeliveryJob
	if err := appDB.Where("reminder_id = ? AND channel = ? AND status = ?", created.ID, ChannelInApp, "pending").First(&followUp).Error; err != nil {
		t.Fatal("expected a follow-up chain job after the 2nd fire")
	}
	if !followUp.ScheduledFor.After(time.Now().UTC()) {
		t.Fatalf("expected follow-up chain job in the future, got %v", followUp.ScheduledFor)
	}

	// 完成后链条取消，不再产生新通知。
	if _, err := completeReminder(appDB, 10, created.ID); err != nil {
		t.Fatal(err)
	}
	dispatchDueJobs(appDB)
	if got := countNotifications(); got != 2 {
		t.Fatalf("expected no extra notification after completion, got %d", got)
	}
}

// 回归：逾期很久后才启用重复提醒时，错过的间隔不逐个补发，只排下一个间隔。
func TestRepeatNotifyCatchUpSkipsMissedIntervals(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	due := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
	created, err := createReminder(appDB, 11, SaveReminderInput{
		Title: "逾期两小时才开启", DueAt: &due, RepeatRule: "none",
		RepeatNotifyMinutes: 5, Channels: []string{ChannelInApp},
	})
	if err != nil {
		t.Fatal(err)
	}

	dispatchDueJobs(appDB)

	var notifications int64
	if err := appDB.Model(&Notification{}).Where("user_id = ? AND reminder_id = ?", 11, created.ID).Count(&notifications).Error; err != nil {
		t.Fatal(err)
	}
	if notifications != 1 {
		t.Fatalf("expected exactly 1 notification (no burst of missed intervals), got %d", notifications)
	}
	var chain DeliveryJob
	if err := appDB.Where("reminder_id = ? AND channel = ? AND status = ?", created.ID, ChannelInApp, "pending").First(&chain).Error; err != nil {
		t.Fatal("expected a follow-up chain job")
	}
	nextIn := time.Until(chain.ScheduledFor)
	if nextIn < 4*time.Minute || nextIn > 6*time.Minute {
		t.Fatalf("expected catch-up chain ~5 minutes out, got %v", nextIn)
	}
}
