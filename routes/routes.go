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
	r.GET("/ws", middlewares.AuthMiddleware(), controllers.ServeWs)

	// Protected routes
	parents := r.Group("/parents")
	parents.Use(middlewares.AuthMiddleware())
	{
		parents.GET("/:firebase_uid", controllers.ReadParent)
		parents.PUT("/:firebase_uid", controllers.UpdateParent)
		parents.DELETE("/:firebase_uid", controllers.DeleteParent)
		parents.POST("/block/apps", controllers.BlockApps)
		parents.POST("/unblock/apps", controllers.UnblockApps)
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
