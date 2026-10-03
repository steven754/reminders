package reminder

import (
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestRecurringReminderQueuesNextOccurrenceAfterSuccess(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	due := time.Now().Add(-time.Minute).Truncate(time.Second)
	created, err := createReminder(appDB, 81, SaveReminderInput{
		Title: "每日提醒", DueAt: &due, RepeatRule: "daily", Channels: []string{ChannelInApp},
	})
	if err != nil {
		t.Fatal(err)
	}
	dispatchDueJobs(appDB)

	var next DeliveryJob
	if err := appDB.Where("reminder_id = ? AND channel = ? AND status = ?", created.ID, ChannelInApp, "pending").First(&next).Error; err != nil {
		t.Fatal("expected next occurrence: ", err)
	}
	gap := next.ScheduledFor.Sub(due.UTC())
	if gap < 23*time.Hour || gap > 25*time.Hour {
		t.Fatalf("expected next daily occurrence about one day later, got %v", gap)
	}
}

func TestSavingDeliveredOccurrenceDoesNotResend(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	due := time.Now().Add(-time.Minute).Truncate(time.Second)
	created, err := createReminder(appDB, 82, SaveReminderInput{
		Title: "保存不补发", DueAt: &due, RepeatRule: "daily", Channels: []string{ChannelInApp},
	})
	if err != nil {
		t.Fatal(err)
	}
	dispatchDueJobs(appDB)

	var notifications int64
	if err := appDB.Model(&Notification{}).Where("user_id = ? AND reminder_id = ?", 82, created.ID).Count(&notifications).Error; err != nil {
		t.Fatal(err)
	}
	if notifications != 1 {
		t.Fatalf("expected one notification before save, got %d", notifications)
	}

	if _, err := updateReminder(appDB, 82, created.ID, SaveReminderInput{
		Title: created.Title, ListID: created.ListID, DueAt: &due, RepeatRule: "daily",
		Channels: []string{ChannelInApp}, Version: created.Version,
	}); err != nil {
		t.Fatal(err)
	}
	dispatchDueJobs(appDB)
	if err := appDB.Model(&Notification{}).Where("user_id = ? AND reminder_id = ?", 82, created.ID).Count(&notifications).Error; err != nil {
		t.Fatal(err)
	}
	if notifications != 1 {
		t.Fatalf("saving a delivered occurrence resent the reminder: got %d notifications", notifications)
	}
}

func TestTerminalFailureStillQueuesRepeatNotifyContinuation(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	due := time.Now().Add(-time.Minute).Truncate(time.Second)
	created, err := createReminder(appDB, 83, SaveReminderInput{
		Title: "失败后继续催办", DueAt: &due, RepeatRule: "none", RepeatNotifyMinutes: 30,
		Channels: []string{ChannelInApp},
	})
	if err != nil {
		t.Fatal(err)
	}
	var job DeliveryJob
	if err := appDB.Where("reminder_id = ? AND channel = ?", created.ID, ChannelInApp).First(&job).Error; err != nil {
		t.Fatal(err)
	}
	if err := appDB.Transaction(func(tx *gorm.DB) error {
		return queueDeliveryContinuation(tx, created.Reminder, job)
	}); err != nil {
		t.Fatal(err)
	}

	var next DeliveryJob
	if err := appDB.Where("reminder_id = ? AND channel = ? AND status = ?", created.ID, ChannelInApp, "pending").
		Order("scheduled_for DESC").First(&next).Error; err != nil {
		t.Fatal("expected repeat-notify continuation: ", err)
	}
	if !next.ScheduledFor.After(time.Now().UTC()) {
		t.Fatalf("expected continuation in the future, got %v", next.ScheduledFor)
	}
}

func TestCleanupDeliveryHistoryKeepsActiveJobs(t *testing.T) {
	cleanup := testDB(t)
	defer cleanup()

	old := time.Now().Add(-100 * 24 * time.Hour)
	terminal := DeliveryJob{
		UserID: 84, ReminderID: 1, Channel: ChannelInApp, ScheduledFor: old, RunAt: old,
		Status: "succeeded", IdempotencyKey: "retention-terminal",
		CreatedAt: old, UpdatedAt: old,
	}
	active := DeliveryJob{
		UserID: 84, ReminderID: 2, Channel: ChannelInApp, ScheduledFor: old, RunAt: old,
		Status: "pending", IdempotencyKey: "retention-active",
		CreatedAt: old, UpdatedAt: old,
	}
	if err := appDB.Create(&terminal).Error; err != nil {
		t.Fatal(err)
	}
	if err := appDB.Create(&active).Error; err != nil {
		t.Fatal(err)
	}
	if err := appDB.Create(&DeliveryAttempt{
		JobID: terminal.ID, AttemptNo: 1, StartedAt: old, FinishedAt: old, Result: "success",
	}).Error; err != nil {
		t.Fatal(err)
	}

	cleanupDeliveryHistory(appDB)
	if err := appDB.First(&DeliveryJob{}, terminal.ID).Error; err == nil {
		t.Fatal("expected old terminal job to be removed")
	}
	if err := appDB.First(&DeliveryJob{}, active.ID).Error; err != nil {
		t.Fatal("active job was removed: ", err)
	}
}
