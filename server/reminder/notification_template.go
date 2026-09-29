package reminder

// notificationTemplates contains the small set of optional, user-selectable
// notification bodies. The reminder notes are appended separately, so each
// template only needs to describe the main sentence.
var notificationTemplates = map[string]string{
	"":       "",
	"title":  "提醒事项：{{title}}",
	"warm":   "温馨提醒：请记得完成「{{title}}」",
	"urgent": "请及时处理：{{title}}",
	"forget": "别忘了：{{title}}",
}

var supportedNotificationTemplates = notificationTemplates
