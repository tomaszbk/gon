package main
import (
 "log/slog"
 "net/http"
)
func nilLogger() {
 var logger *slog.Logger
 var emit func() = () => { logger.Info("unsafe") }
 emit()
}
func nilServer() {
 var server *http.Server
 var inspect func() = () => { server.SetKeepAlivesEnabled(false) }
 inspect()
}
func main() { nilLogger(); nilServer() }
