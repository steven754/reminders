package reminder

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"smallgo/server/sysconfig"
)

// SyncMedkitExpiryReminder creates or updates the ordinary Reminder used for
// one Medkit medicine. It deliberately goes through the existing reminder
// delivery tables, so Bark, email, in-app and every other configured channel
// keep exactly the same delivery/retry behavior as a normal reminder.
func SyncMedkitExpiryReminder(tx *gorm.DB, ownerID uint, reminderID *uint, medicineName, medicineNotes, expiryDate string, enabled bool, days int, reminderTime string, requestedChannels []string, requestedTargets map[string][]uint) (uint, error) {
	if !enabled || strings.TrimSpace(expiryDate) == "" {
		if reminderID != nil && *reminderID > 0 {
			if err := retireMedkitReminder(tx, ownerID, *reminderID); err != nil {
				return 0, err
			}
		}
		return 0, nil
	}
	if days < 1 || days > 365 {
		return 0, errors.New("到期提醒提前天数应为 1–365 天")
	}
	if _, err := time.Parse("15:04", reminderTime); err != nil {
		return 0, errors.New("到期提醒时间格式无效")
	}
	expiry, err := time.ParseInLocation("2006-01-02", expiryDate, medkitLocation())
	if err != nil {
		return 0, errors.New("到期日期格式无效")
	}
	clock, _ := time.ParseInLocation("15:04", reminderTime, medkitLocation())
	dueAt := time.Date(expiry.Year(), expiry.Month(), expiry.Day(), clock.Hour(), clock.Minute(), 0, 0, medkitLocation()).AddDate(0, 0, -days)

	channels, targets, err := medkitReminderChannels(tx, ownerID, requestedChannels, requestedTargets)
	if err != nil {
		return 0, err
	}
	notes := strings.TrimSpace(medicineNotes)
	if notes != "" {
		notes += "\n"
	}
	notes += fmt.Sprintf("药品到期日期：%s（提前 %d 天提醒）", expiryDate, days)
	in := SaveReminderInput{
		Title: medicineName + "到期提醒", Notes: notes, DueAt: &dueAt,
		RepeatRule: "none", Calendar: "solar", Channels: channels,
		ChannelTargets: targets,
	}
	if err := validateReminderInput(&in); err != nil {
		return 0, err
	}

	var reminder Reminder
	if reminderID != nil && *reminderID > 0 {
		if err := tx.Where("id = ? AND user_id = ?", *reminderID, ownerID).First(&reminder).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, err
		}
	}
	if reminder.ID == 0 {
		list, err := ensureDefaultList(tx, ownerID)
		if err != nil {
			return 0, err
		}
		reminder = Reminder{UserID: ownerID, ListID: list.ID, Title: in.Title, Notes: in.Notes, DueAt: in.DueAt, AllDay: false, RepeatRule: "none", Calendar: "solar", Version: 1}
		if err := tx.Create(&reminder).Error; err != nil {
			return 0, err
		}
	} else {
		reminder.Title, reminder.Notes, reminder.DueAt = in.Title, in.Notes, in.DueAt
		reminder.EndAt, reminder.CompletedAt, reminder.SnoozedUntil = nil, nil, nil
		reminder.RepeatRule, reminder.CronExpr, reminder.Calendar = "none", "", "solar"
		reminder.Version++
		if err := tx.Save(&reminder).Error; err != nil {
			return 0, err
		}
	}
	if err := replaceChannels(tx, reminder, in); err != nil {
		return 0, err
	}
	if err := rebuildJobs(tx, reminder, in.Channels); err != nil {
		return 0, err
	}
	return reminder.ID, nil
}

func retireMedkitReminder(tx *gorm.DB, ownerID, reminderID uint) error {
	var reminder Reminder
	if err := tx.Where("id = ? AND user_id = ?", reminderID, ownerID).First(&reminder).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if err := tx.Model(&DeliveryJob{}).Where("reminder_id = ? AND user_id = ? AND status IN ?", reminderID, ownerID, []string{"pending", "retry"}).Update("status", "cancelled").Error; err != nil {
		return err
	}
	now := time.Now()
	if err := retireDueNotifications(tx, ownerID, reminderID, now); err != nil {
		return err
	}
	return tx.Model(&reminder).Update("deleted_at", &now).Error
}

func medkitReminderChannels(tx *gorm.DB, userID uint, requested []string, requestedTargets map[string][]uint) ([]string, map[string][]uint, error) {
	if len(requested) == 0 {
		raw, err := sysconfig.GetConfig(tx, "reminder_default_channels", userID)
		if err == nil {
			requested = strings.Split(raw, ",")
		}
	}
	if len(requested) == 0 {
		requested = []string{ChannelInApp}
	}

	channels := make([]string, 0, len(requested))
	targets := map[string][]uint{}
	seen := map[string]bool{}
	for _, channel := range requested {
		channel = strings.ToLower(strings.TrimSpace(channel))
		if channel == "" || seen[channel] {
			continue
		}
		if !supportedChannels[channel] {
			return nil, nil, fmt.Errorf("不支持的提醒渠道：%s", channel)
		}
		seen[channel] = true
		channels = append(channels, channel)
		if channel == ChannelInApp {
			continue
		}
		var activeIDs []uint
		if err := tx.Model(&ChannelBinding{}).
			Where("user_id = ? AND channel = ? AND status = ?", userID, channel, "active").
			Order("id ASC").Pluck("id", &activeIDs).Error; err != nil || len(activeIDs) == 0 {
			if err != nil {
				return nil, nil, err
			}
			return nil, nil, fmt.Errorf("%s尚未配置有效的接收目标，请先到通知方式中完成配置", channelLabel(channel))
		}

		wanted, hasSelection := requestedTargets[channel]
		if !hasSelection {
			targets[channel] = activeIDs
			continue
		}
		wanted = dedupeUint(wanted)
		if len(wanted) == 0 {
			return nil, nil, fmt.Errorf("%s提醒至少选择一个接收者", channelLabel(channel))
		}
		valid := make(map[uint]bool, len(activeIDs))
		for _, id := range activeIDs {
			valid[id] = true
		}
		for _, id := range wanted {
			if !valid[id] {
				return nil, nil, fmt.Errorf("%s提醒选择的接收者不存在或已停用", channelLabel(channel))
			}
		}
		targets[channel] = wanted
	}
	if len(channels) == 0 {
		return nil, nil, errors.New("至少选择一种通知方式")
	}
	return channels, targets, nil
}

func medkitLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	return loc
}
