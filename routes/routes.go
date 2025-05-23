package routes

import (
	"PinguinMobile/controllers"
	"PinguinMobile/middlewares"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine) {
	// Public routes
	r.POST("/register/parent", controllers.RegisterParent)
	r.POST("/register/child", controllers.RegisterChild)
	r.POST("/login/parent", controllers.LoginParent)
	r.POST("/login/child", controllers.LoginChild) // Add this line
	r.POST("/auth/token-verify", controllers.TokenVerify)
	// Маршрут WebSocket (проверьте, что он есть)
	r.GET("/ws", controllers.ServeWs)
	r.GET("/debug/auth", middlewares.AuthMiddleware(), controllers.DebugAuth)
	r.GET("/translations", controllers.GetTranslations)
	r.POST("/auth/verify-email", middlewares.AuthMiddleware(), controllers.VerifyParentEmail)
	r.POST("/auth/resend-verification", middlewares.AuthMiddleware(), controllers.ResendVerificationCode)
	r.DELETE("/:firebase_uid", controllers.DeleteParent)
	r.POST("/auth/forgot-password", controllers.ForgotPassword)
	r.POST("/auth/reset-password", controllers.ResetPassword)                                 // По коду из email
	r.POST("/auth/change-password", middlewares.AuthMiddleware(), controllers.ChangePassword) // Если пользователь авторизован

	// Protected routes
	parents := r.Group("/parents")
	parents.Use(middlewares.AuthMiddleware())
	{
		parents.GET("/:firebase_uid", controllers.ReadParent)
		parents.PUT("/:firebase_uid", controllers.UpdateParent)
		// parents.DELETE("/:firebase_uid", controllers.DeleteParent)
		// parents.POST("/block/apps", controllers.BlockApps)
		// parents.POST("/unblock/apps", controllers.UnblockApps)

		// Новые маршруты для временной блокировки
		parents.GET("/block/apps/time/:firebase_uid", controllers.GetTimeBlockedApps)
		parents.POST("/apps/time-rules", controllers.ManageAppTimeRules)
		// Маршруты для одноразовой блокировки
		// parents.POST("/:firebase_uid/block-apps-once", controllers.BlockAppsTempOnce)
		parents.GET("/block/apps/onetime/:firebase_uid", controllers.GetOneTimeBlocks) // Новый единый маршрут
		parents.POST("/apps/onetime-rules", controllers.ManageOneTimeRules)

		// parents.DELETE("/:firebase_uid/block-apps-once/:child_id", controllers.CancelOneTimeBlocks)
	}

	// Separate route group for unbind and monitor routes to avoid conflicts
	parentsUnbind := r.Group("/parents/unbind")
	parentsUnbind.Use(middlewares.AuthMiddleware())
	{
		parentsUnbind.DELETE("/", controllers.UnbindChild)
	}

	// Separate route group for monitor routes to avoid conflicts
	parentsMonitor := r.Group("/parents/monitor")
	parentsMonitor.Use(middlewares.AuthMiddleware())
	{
		parentsMonitor.POST("/", controllers.MonitorChildrenUsage)
		parentsMonitor.POST("/child", controllers.MonitorChildUsage)
	}

	// Define the new routes for children
	children := r.Group("/children")
	children.Use(middlewares.AuthMiddleware())
	{
		children.GET("/:firebase_uid", controllers.ReadChild)
		children.PUT("/:firebase_uid", controllers.UpdateChild)
		children.DELETE("/:firebase_uid", controllers.DeleteChild)
		children.POST("/:firebase_uid/logout", controllers.LogoutChild)
		children.POST("/:firebase_uid/monitor", controllers.MonitorChild)
		children.POST("/rebind", controllers.RebindChild)

		// Новый маршрут для проверки блокировки
		children.GET("/check-blocking", controllers.CheckAppBlocking)
	}

	// Chat routes
	chat := r.Group("/chat")
	chat.Use(middlewares.AuthMiddleware())
	{
		chat.POST("/messages/text", controllers.SendTextMessage)
		chat.POST("/messages/media", controllers.SendMediaMessage)
		chat.GET("/family/:parent_id/messages", controllers.GetFamilyMessages)
		chat.GET("/private/:parent_id/:user_id", controllers.GetPrivateMessages)
		chat.PUT("/messages/read", controllers.MarkAsRead)
		chat.DELETE("/messages/:message_id", controllers.DeleteMessage)
		chat.POST("/moderation", controllers.ModerateMessage)
		chat.GET("/family/:parent_id/unread", controllers.GetUnreadCount)
		chat.GET("/private/:parent_id/unread", controllers.GetUnreadPrivateCount)
		chat.GET("/family/:parent_id/channels", controllers.GetChannelsList)
	}
}
