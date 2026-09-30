package medkit

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"smallgo/server/audit"
	"smallgo/server/database"
	"smallgo/server/reminder"
	"smallgo/server/response"
	"smallgo/server/sysconfig"
)

var (
	errSharingDisabled   = errors.New("管理员尚未开启 Medkit 用户关联")
	errMedkitForbidden   = errors.New("没有权限操作这个药箱")
	errInvalidPermission = errors.New("无效的药箱权限")
)

func currentUserID(c *gin.Context) uint {
	return c.GetUint("userID")
}

func sharingEnabled(db *gorm.DB) bool {
	value, err := sysconfig.GetConfig(db, "medkit_user_sharing_enabled", 0)
	return err == nil && value == "true"
}

func permissionAllowsEdit(permission string) bool {
	return permission == "owner" || permission == PermissionEdit
}

// resolveOwner checks the selected owner's Medkit access without ever
// accepting a reminder user's ownership or notification permissions.
func resolveOwner(db *gorm.DB, memberID, ownerID uint) (string, error) {
	if ownerID == 0 {
		ownerID = memberID
	}
	if ownerID == memberID {
		return "owner", nil
	}
	if !sharingEnabled(db) {
		return "", errSharingDisabled
	}
	var access Access
	if err := db.Where("owner_id = ? AND member_id = ?", ownerID, memberID).First(&access).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", errMedkitForbidden
		}
		return "", err
	}
	return access.Permission, nil
}

func parseID(c *gin.Context) (uint, bool) {
	raw := strings.TrimSpace(c.Param("id"))
	n, err := strconv.ParseUint(raw, 10, 32)
	if err != nil || n == 0 {
		response.ErrorBadRequest(c, "无效的 Medkit 数据 ID")
		return 0, false
	}
	return uint(n), true
}

func mapMedkitError(c *gin.Context, err error, notFound string) {
	switch {
	case errors.Is(err, errSharingDisabled), errors.Is(err, errMedkitForbidden):
		response.ErrorForbidden(c, err.Error())
	case errors.Is(err, gorm.ErrRecordNotFound):
		response.Error(c, http.StatusNotFound, response.CodeNotFound, notFound)
	default:
		response.ErrorInternal(c, "Medkit 操作失败")
	}
}

func handleContext(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := currentUserID(c)
		var current database.User
		if err := db.Select("id", "username").First(&current, userID).Error; err != nil {
			response.ErrorInternal(c, "读取当前用户失败")
			return
		}

		owners := []OwnerView{{ID: current.ID, Username: current.Username, Permission: "owner", CanEdit: true, CanManageUsers: sharingEnabled(db)}}
		var rows []struct {
			Access
			Username string `gorm:"column:username"`
		}
		if err := db.Table("medkit_accesses AS a").
			Select("a.*, u.username").
			Joins("JOIN users AS u ON u.id = a.owner_id").
			Where("a.member_id = ? AND u.status = ?", userID, 1).
			Order("u.username ASC").Scan(&rows).Error; err != nil {
			response.ErrorInternal(c, "读取可访问药箱失败")
			return
		}
		for _, row := range rows {
			owners = append(owners, OwnerView{ID: row.OwnerID, Username: row.Username, Permission: row.Permission, CanEdit: permissionAllowsEdit(row.Permission), CanManageUsers: false})
		}

		var managed []AccessView
		if sharingEnabled(db) {
			var accessRows []Access
			if err := db.Where("owner_id = ?", userID).Order("id ASC").Find(&accessRows).Error; err != nil {
				response.ErrorInternal(c, "读取关联用户失败")
				return
			}
			for _, access := range accessRows {
				var member database.User
				if err := db.Select("id", "username", "status").First(&member, access.MemberID).Error; err != nil || member.Status != 1 {
					continue
				}
				managed = append(managed, AccessView{Access: access, MemberUsername: member.Username})
			}
		}

		response.Success(c, gin.H{
			"sharing_enabled": sharingEnabled(db),
			"current_user":    gin.H{"id": current.ID, "username": current.Username},
			"owners":          owners,
			"managed_access":  managed,
		})
	}
}

func handleSearchUsers(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !sharingEnabled(db) {
			response.Success(c, []interface{}{})
			return
		}
		query := strings.TrimSpace(c.Query("q"))
		if query == "" {
			response.Success(c, []interface{}{})
			return
		}
		var users []database.User
		if err := db.Select("id", "username").
			Where("id <> ? AND status = ? AND username LIKE ?", currentUserID(c), 1, "%"+query+"%").
			Order("username ASC").Limit(20).Find(&users).Error; err != nil {
			response.ErrorInternal(c, "搜索用户失败")
			return
		}
		response.Success(c, users)
	}
}

func handleMedkitChannelStatuses(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ownerID := selectedOwner(c)
		if ownerID == 0 {
			response.ErrorBadRequest(c, "无效的药箱用户")
			return
		}
		permission, err := resolveOwner(db, currentUserID(c), ownerID)
		if err != nil {
			mapMedkitError(c, err, "")
			return
		}
		statuses := reminder.ChannelStatusesForUser(db, ownerID)
		// ChannelStatus normally includes the authenticated user's targets. A
		// Medkit member only needs to know whether a method is usable; never
		// expose the owner's addresses, webhook URLs, or binding IDs here.
		if permission != "owner" {
			for i := range statuses {
				statuses[i].TargetMasked = ""
				statuses[i].Bindings = nil
			}
		}
		response.Success(c, statuses)
	}
}

func handleListAccess(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !sharingEnabled(db) {
			response.Success(c, []AccessView{})
			return
		}
		var rows []Access
		if err := db.Where("owner_id = ?", currentUserID(c)).Order("id ASC").Find(&rows).Error; err != nil {
			response.ErrorInternal(c, "读取关联用户失败")
			return
		}
		result := make([]AccessView, 0, len(rows))
		for _, row := range rows {
			var member database.User
			if err := db.Select("id", "username", "status").First(&member, row.MemberID).Error; err != nil || member.Status != 1 {
				continue
			}
			result = append(result, AccessView{Access: row, MemberUsername: member.Username})
		}
		response.Success(c, result)
	}
}

func handleCreateAccess(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !sharingEnabled(db) {
			response.ErrorForbidden(c, errSharingDisabled.Error())
			return
		}
		var in struct {
			Username   string `json:"username"`
			Permission string `json:"permission"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			response.ErrorBadRequest(c, "关联用户参数无效")
			return
		}
		in.Username = strings.TrimSpace(in.Username)
		if in.Username == "" {
			response.ErrorBadRequest(c, "请输入用户名")
			return
		}
		if in.Permission == "" {
			in.Permission = PermissionView
		}
		if in.Permission != PermissionView && in.Permission != PermissionEdit {
			response.ErrorBadRequest(c, errInvalidPermission.Error())
			return
		}
		var member database.User
		if err := db.Where("username = ? AND status = ?", in.Username, 1).First(&member).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				response.Error(c, http.StatusNotFound, response.CodeNotFound, "用户不存在或已停用")
				return
			}
			response.ErrorInternal(c, "读取用户失败")
			return
		}
		ownerID := currentUserID(c)
		if member.ID == ownerID {
			response.ErrorBadRequest(c, "不能关联自己")
			return
		}
		access := Access{OwnerID: ownerID, MemberID: member.ID, Permission: in.Permission}
		if err := db.Where("owner_id = ? AND member_id = ?", ownerID, member.ID).
			Assign(map[string]interface{}{"permission": in.Permission}).FirstOrCreate(&access).Error; err != nil {
			response.ErrorInternal(c, "保存关联用户失败")
			return
		}
		audit.Log(db, c, "medkit_access_grant", "medkit_access", access.ID, fmt.Sprintf("将用户 %s 关联到自己的药箱，权限：%s", member.Username, permissionLabel(in.Permission)))
		response.Success(c, gin.H{"id": access.ID, "member_username": member.Username, "permission": access.Permission})
	}
}

func handleDeleteAccess(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		var access Access
		if err := db.First(&access, id).Error; err != nil {
			mapMedkitError(c, err, "关联记录不存在")
			return
		}
		userID := currentUserID(c)
		if !sharingEnabled(db) || (access.OwnerID != userID && access.MemberID != userID) {
			response.ErrorForbidden(c, errMedkitForbidden.Error())
			return
		}
		if err := db.Delete(&access).Error; err != nil {
			response.ErrorInternal(c, "移除关联用户失败")
			return
		}
		audit.Log(db, c, "medkit_access_revoke", "medkit_access", access.ID, "移除 Medkit 用户关联")
		response.Success(c, gin.H{"deleted": true})
	}
}

func validateMedicineInput(name, expiryDate string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("药品名称不能为空")
	}
	if len([]rune(strings.TrimSpace(name))) > 120 {
		return errors.New("药品名称不能超过 120 个字符")
	}
	if expiryDate != "" {
		if _, err := time.Parse("2006-01-02", expiryDate); err != nil {
			return errors.New("到期日期格式无效")
		}
	}
	return nil
}

func normalizeReminderFields(in *Medicine) error {
	if in.ReminderTime == "" {
		in.ReminderTime = "09:00"
	}
	if in.ReminderDays == 0 {
		in.ReminderDays = 7
	}
	if _, err := time.Parse("15:04", in.ReminderTime); err != nil {
		return errors.New("提醒时间格式无效")
	}
	if in.ReminderEnabled {
		if strings.TrimSpace(in.ExpiryDate) == "" {
			return errors.New("启用到期提醒前请先填写到期日期")
		}
		if in.ReminderDays < 1 || in.ReminderDays > 365 {
			return errors.New("提前提醒天数应为 1–365 天")
		}
	}
	return nil
}

func selectedOwner(c *gin.Context) uint {
	raw := strings.TrimSpace(c.Query("owner_id"))
	if raw == "" {
		return currentUserID(c)
	}
	id, err := strconv.ParseUint(raw, 10, 32)
	if err != nil || id == 0 {
		return 0
	}
	return uint(id)
}

func handleListMedicines(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ownerID := selectedOwner(c)
		if ownerID == 0 {
			response.ErrorBadRequest(c, "无效的药箱用户")
			return
		}
		permission, err := resolveOwner(db, currentUserID(c), ownerID)
		if err != nil {
			mapMedkitError(c, err, "")
			return
		}
		query := db.Where("owner_id = ?", ownerID)
		if search := strings.TrimSpace(c.Query("q")); search != "" {
			like := "%" + search + "%"
			query = query.Where("name LIKE ? OR generic_name LIKE ? OR specification LIKE ? OR notes LIKE ?", like, like, like, like)
		}
		var medicines []Medicine
		if err := query.
			Order("CASE WHEN expiry_date = '' THEN 1 ELSE 0 END, expiry_date ASC, id DESC").Find(&medicines).Error; err != nil {
			response.ErrorInternal(c, "读取药品失败")
			return
		}
		if err := attachReminderChannels(db, ownerID, medicines); err != nil {
			response.ErrorInternal(c, "读取药品通知方式失败")
			return
		}
		if permission != "owner" {
			for i := range medicines {
				medicines[i].ReminderChannelTargets = nil
			}
		}
		response.Success(c, medicines)
	}
}

// attachReminderChannels exposes the channels of the generated ordinary
// reminder without duplicating them in the Medkit schema. This keeps a
// medicine's notification choice in the same channel rows used by normal
// reminders and preserves the existing delivery behavior.
func attachReminderChannels(db *gorm.DB, ownerID uint, medicines []Medicine) error {
	ids := make([]uint, 0, len(medicines))
	for _, medicine := range medicines {
		if medicine.ReminderID != nil && *medicine.ReminderID > 0 {
			ids = append(ids, *medicine.ReminderID)
		}
	}
	byReminder := make(map[uint][]string, len(ids))
	byReminderTargets := make(map[uint]map[string][]uint, len(ids))
	if len(ids) > 0 {
		var rows []reminder.ReminderChannel
		if err := db.Where("reminder_id IN ? AND user_id = ? AND enabled = ?", ids, ownerID, true).
			Order("id ASC").Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			byReminder[row.ReminderID] = append(byReminder[row.ReminderID], row.Channel)
			if row.Targets != "" {
				if targets := parseReminderTargets(row.Targets); targets != nil {
					if existing, ok := byReminderTargets[row.ReminderID]; ok {
						existing[row.Channel] = targets
					} else {
						byReminderTargets[row.ReminderID] = map[string][]uint{row.Channel: targets}
					}
				}
			}
		}
	}
	for i := range medicines {
		medicines[i].ReminderChannels = []string{}
		medicines[i].ReminderChannelTargets = map[string][]uint{}
		if medicines[i].ReminderID != nil {
			medicines[i].ReminderChannels = byReminder[*medicines[i].ReminderID]
			if targets, ok := byReminderTargets[*medicines[i].ReminderID]; ok {
				medicines[i].ReminderChannelTargets = targets
			}
		}
		if medicines[i].ReminderChannels == nil {
			medicines[i].ReminderChannels = []string{}
		}
	}
	return nil
}

func parseReminderTargets(raw string) []uint {
	var targets []uint
	if err := json.Unmarshal([]byte(raw), &targets); err != nil {
		return nil
	}
	return targets
}

func handleCreateMedicine(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in Medicine
		if err := c.ShouldBindJSON(&in); err != nil {
			response.ErrorBadRequest(c, "药品参数无效")
			return
		}
		if in.OwnerID == 0 {
			in.OwnerID = currentUserID(c)
		}
		permission, err := resolveOwner(db, currentUserID(c), in.OwnerID)
		if err != nil {
			mapMedkitError(c, err, "")
			return
		}
		if !permissionAllowsEdit(permission) {
			response.ErrorForbidden(c, errMedkitForbidden.Error())
			return
		}
		if err := validateMedicineInput(in.Name, in.ExpiryDate); err != nil {
			response.ErrorBadRequest(c, err.Error())
			return
		}
		in.ID = 0
		in.ReminderID = nil
		in.Name = strings.TrimSpace(in.Name)
		in.GenericName = strings.TrimSpace(in.GenericName)
		in.Specification = strings.TrimSpace(in.Specification)
		in.Quantity = strings.TrimSpace(in.Quantity)
		in.Unit = strings.TrimSpace(in.Unit)
		in.ExpiryDate = strings.TrimSpace(in.ExpiryDate)
		in.Notes = strings.TrimSpace(in.Notes)
		if err := normalizeReminderFields(&in); err != nil {
			response.ErrorBadRequest(c, err.Error())
			return
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&in).Error; err != nil {
				return err
			}
			reminderID, err := reminder.SyncMedkitExpiryReminder(tx, in.OwnerID, in.ReminderID, in.Name, in.Notes, in.ExpiryDate, in.ReminderEnabled, in.ReminderDays, in.ReminderTime, in.ReminderChannels, in.ReminderChannelTargets)
			if err != nil {
				return err
			}
			if reminderID > 0 {
				in.ReminderID = &reminderID
				return tx.Model(&in).Update("reminder_id", reminderID).Error
			}
			in.ReminderID = nil
			return nil
		}); err != nil {
			response.ErrorBadRequest(c, err.Error())
			return
		}
		items := []Medicine{in}
		if err := attachReminderChannels(db, in.OwnerID, items); err != nil {
			response.ErrorInternal(c, "读取药品通知方式失败")
			return
		}
		in.ReminderChannels = items[0].ReminderChannels
		in.ReminderChannelTargets = items[0].ReminderChannelTargets
		if permission != "owner" {
			in.ReminderChannelTargets = nil
		}
		audit.Log(db, c, "medkit_medicine_create", "medkit_medicine", in.ID, fmt.Sprintf("为 %s 的药箱新增药品：%s", ownerUsername(db, in.OwnerID), in.Name))
		response.Success(c, in)
	}
}

func findMedicine(db *gorm.DB, id uint) (Medicine, error) {
	var medicine Medicine
	err := db.First(&medicine, id).Error
	return medicine, err
}

func handleUpdateMedicine(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		medicine, err := findMedicine(db, id)
		if err != nil {
			mapMedkitError(c, err, "药品不存在")
			return
		}
		permission, err := resolveOwner(db, currentUserID(c), medicine.OwnerID)
		if err != nil {
			mapMedkitError(c, err, "")
			return
		}
		if !permissionAllowsEdit(permission) {
			response.ErrorForbidden(c, errMedkitForbidden.Error())
			return
		}
		var in Medicine
		if err := c.ShouldBindJSON(&in); err != nil {
			response.ErrorBadRequest(c, "药品参数无效")
			return
		}
		if err := validateMedicineInput(in.Name, in.ExpiryDate); err != nil {
			response.ErrorBadRequest(c, err.Error())
			return
		}
		updates := map[string]interface{}{
			"name": strings.TrimSpace(in.Name), "generic_name": strings.TrimSpace(in.GenericName),
			"specification": strings.TrimSpace(in.Specification), "quantity": strings.TrimSpace(in.Quantity),
			"unit": strings.TrimSpace(in.Unit), "expiry_date": strings.TrimSpace(in.ExpiryDate),
			"notes": strings.TrimSpace(in.Notes), "reminder_enabled": in.ReminderEnabled,
			"reminder_days": in.ReminderDays, "reminder_time": strings.TrimSpace(in.ReminderTime),
		}
		in.ReminderTime = strings.TrimSpace(in.ReminderTime)
		if err := normalizeReminderFields(&in); err != nil {
			response.ErrorBadRequest(c, err.Error())
			return
		}
		updates["reminder_days"] = in.ReminderDays
		updates["reminder_time"] = in.ReminderTime
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(&medicine).Updates(updates).Error; err != nil {
				return err
			}
			reminderID, err := reminder.SyncMedkitExpiryReminder(tx, medicine.OwnerID, medicine.ReminderID, strings.TrimSpace(in.Name), strings.TrimSpace(in.Notes), strings.TrimSpace(in.ExpiryDate), in.ReminderEnabled, in.ReminderDays, in.ReminderTime, in.ReminderChannels, in.ReminderChannelTargets)
			if err != nil {
				return err
			}
			if reminderID > 0 {
				medicine.ReminderID = &reminderID
				return tx.Model(&medicine).Update("reminder_id", reminderID).Error
			}
			medicine.ReminderID = nil
			return tx.Model(&medicine).Update("reminder_id", nil).Error
		}); err != nil {
			response.ErrorBadRequest(c, err.Error())
			return
		}
		medicine.Name = updates["name"].(string)
		medicine.GenericName = updates["generic_name"].(string)
		medicine.Specification = updates["specification"].(string)
		medicine.Quantity = updates["quantity"].(string)
		medicine.Unit = updates["unit"].(string)
		medicine.ExpiryDate = updates["expiry_date"].(string)
		medicine.Notes = updates["notes"].(string)
		medicine.ReminderEnabled = in.ReminderEnabled
		medicine.ReminderDays = in.ReminderDays
		medicine.ReminderTime = in.ReminderTime
		items := []Medicine{medicine}
		if err := attachReminderChannels(db, medicine.OwnerID, items); err != nil {
			response.ErrorInternal(c, "读取药品通知方式失败")
			return
		}
		medicine.ReminderChannels = items[0].ReminderChannels
		medicine.ReminderChannelTargets = items[0].ReminderChannelTargets
		if permission != "owner" {
			medicine.ReminderChannelTargets = nil
		}
		audit.Log(db, c, "medkit_medicine_update", "medkit_medicine", medicine.ID, fmt.Sprintf("更新 %s 的药品：%s", ownerUsername(db, medicine.OwnerID), medicine.Name))
		response.Success(c, medicine)
	}
}

func handleDeleteMedicine(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		medicine, err := findMedicine(db, id)
		if err != nil {
			mapMedkitError(c, err, "药品不存在")
			return
		}
		permission, err := resolveOwner(db, currentUserID(c), medicine.OwnerID)
		if err != nil {
			mapMedkitError(c, err, "")
			return
		}
		if !permissionAllowsEdit(permission) {
			response.ErrorForbidden(c, errMedkitForbidden.Error())
			return
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			if _, err := reminder.SyncMedkitExpiryReminder(tx, medicine.OwnerID, medicine.ReminderID, medicine.Name, medicine.Notes, medicine.ExpiryDate, false, medicine.ReminderDays, medicine.ReminderTime, nil, nil); err != nil {
				return err
			}
			return tx.Delete(&medicine).Error
		}); err != nil {
			response.ErrorInternal(c, "删除药品失败")
			return
		}
		audit.Log(db, c, "medkit_medicine_delete", "medkit_medicine", medicine.ID, fmt.Sprintf("删除 %s 的药品：%s", ownerUsername(db, medicine.OwnerID), medicine.Name))
		response.Success(c, gin.H{"deleted": true})
	}
}

func ownerUsername(db *gorm.DB, userID uint) string {
	var user database.User
	if err := db.Select("username").First(&user, userID).Error; err != nil {
		return fmt.Sprintf("用户 #%d", userID)
	}
	return user.Username
}

func permissionLabel(permission string) string {
	if permission == PermissionEdit {
		return "可编辑"
	}
	return "只读"
}
