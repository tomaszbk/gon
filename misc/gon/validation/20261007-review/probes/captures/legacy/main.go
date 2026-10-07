package main
import (
 "io"
 "log/slog"
 "net/http"
)
func safeLogger() {
 logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
 var emit func() = func() { logger.Info("safe") }
 emit()
}
func safeServer() {
 server := &http.Server{Addr: "capture"}
 var inspect func() = func() { server.SetKeepAlivesEnabled(false) }
 inspect()
}
func nilLogger() {
 var logger *slog.Logger
 var emit func() = func() { logger.Info("unsafe") }
 emit()
}
func nilServer() {
 var server *http.Server
 var inspect func() = func() { server.SetKeepAlivesEnabled(false) }
 inspect()
}
func expectPanic(f func()) {
 panicked := false
 func() { defer func() { panicked = recover() != nil }(); f() }()
 if !panicked { panic("missing nil capture panic") }
}
func main() {
 safeLogger(); safeServer()
 expectPanic(nilLogger); expectPanic(nilServer)
 println("capture pair: safe + 2 nil panics")
}
