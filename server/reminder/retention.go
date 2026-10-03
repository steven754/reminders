package reminder

import (
	"time"

	"smallgo/server/logger"

	"gorm.io/gorm"
)

const (
	deliveryHistoryRetention = 90 * 24 * time.Hour
	deliveryCleanupBatchSize = 2000
)

var terminalDeliveryStatuses = []string{"succeeded", "failed", "cancelled"}

// cleanupDeliveryHistory removes only terminal delivery history. Active jobs
// are never touched, and attempts are removed together with their job so the
// admin delivery list does not retain dangling history indefinitely.
func cleanupDeliveryHistory(db *gorm.DB) {
	cutoff := time.Now().UTC().Add(-deliveryHistoryRetention)
	var ids []uint
	if err := db.Model(&DeliveryJob{}).
		Where("status IN ? AND updated_at < ?", terminalDeliveryStatuses, cutoff).
		Order("id ASC").Limit(deliveryCleanupBatchSize).Pluck("id", &ids).Error; err != nil {
		logger.Error("reminder: scan delivery history for cleanup failed: %v", err)
		return
	}
	if len(ids) == 0 {
		return
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("job_id IN ?", ids).Delete(&DeliveryAttempt{}).Error; err != nil {
			return err
		}
		return tx.Where("id IN ?", ids).Delete(&DeliveryJob{}).Error
	}); err != nil {
		logger.Error("reminder: cleanup delivery history failed: %v", err)
		return
	}
	logger.Info("reminder: removed %d delivery jobs older than %d days", len(ids), int(deliveryHistoryRetention/(24*time.Hour)))
}
