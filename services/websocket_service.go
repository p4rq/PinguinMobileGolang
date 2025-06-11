package services

// WebSocketHubInterface определяет интерфейс Hub
type WebSocketHubInterface interface {
	NotifyLimitChange(parentID string, childToken string)
}

// WebSocketHub глобальная ссылка на WebSocket Hub
var WebSocketHub WebSocketHubInterface

// SetWebSocketHub устанавливает глобальную ссылку на WebSocket Hub
func SetWebSocketHub(hub WebSocketHubInterface) {
	WebSocketHub = hub
	println("[WEBSOCKET] WebSocket Hub установлен в сервисах")
}
