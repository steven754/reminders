package medkit

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"smallgo/server/apps"
	"smallgo/server/database"
	"smallgo/server/sysconfig"
)

func init() {
	database.RegisterModels(&Medicine{}, &Access{}, &AIConfig{})

	sysconfig.RegisterConfig(sysconfig.ConfigDef{
		Key: "medkit_user_sharing_enabled", Scope: sysconfig.ScopeSystem, Type: sysconfig.TypeBool,
		Default: "false", Group: "medkit", Label: "允许 Medkit 关联用户",
		Description: "仅控制 AI 药箱的跨用户访问；关闭后每个人只能操作自己的药箱，不影响普通提醒。",
	})

	apps.Register(apps.App{
		Name:        "medkit",
		DisplayName: "AI 药箱",
		Icon:        "heart-pulse",
		RoutePrefix: "/medkit",
		NavPosition: 20,
		SetupAuth:   setupAuthRoutes,
		SetupAdmin:  setupAdminRoutes,
	})
}

func setupAuthRoutes(api *gin.RouterGroup, db *gorm.DB) {
	group := api.Group("/medkit")
	group.GET("/context", handleContext(db))
	group.GET("/users", handleSearchUsers(db))
	group.GET("/channels", handleMedkitChannelStatuses(db))
	group.GET("/access", handleListAccess(db))
	group.POST("/access", handleCreateAccess(db))
	group.DELETE("/access/:id", handleDeleteAccess(db))
	group.GET("/medicines", handleListMedicines(db))
	group.POST("/medicines", handleCreateMedicine(db))
	group.PUT("/medicines/:id", handleUpdateMedicine(db))
	group.DELETE("/medicines/:id", handleDeleteMedicine(db))
	group.POST("/ai/parse", handleAIParse(db))
}

func setupAdminRoutes(api *gin.RouterGroup, db *gorm.DB) {
	api.GET("/medkit/admin/ai", handleGetAIConfig(db))
	api.PUT("/medkit/admin/ai", handleSaveAIConfig(db))
}
