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
	r.POST("/auth/token-verify", controllers.TokenVerify)

	// Protected routes
	parents := r.Group("/parents")
	parents.Use(middlewares.AuthMiddleware())
	{
		parents.GET("/:firebase_uid", controllers.ReadParent)
		parents.PUT("/:firebase_uid", controllers.UpdateParent)
		parents.DELETE("/:firebase_uid", controllers.DeleteParent)
	}

	// Separate route group for unbind and monitor routes to avoid conflicts
	parentsUnbind := r.Group("/parents/unbind")
	parentsUnbind.Use(middlewares.AuthMiddleware())
	{
		parentsUnbind.DELETE("/:parentFirebaseUid/:childFirebaseUid", controllers.UnbindChild)
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
		children.POST("/:firebase_uid/rebind", controllers.RebindChild)
	}
}
