package main
import (
 "io"
 "log/slog"
 "net/http"
 "strconv"
)
func safeLogger() {
 logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
 strconv.Atoi("1") or err { logger.Error("failed", "error", err); return }
 var emit func() = () => { logger.Info("safe") }
 go emit()
}
func safeServer() {
 server := &http.Server{Addr: "capture"}
 strconv.Atoi("1") or err { return }
 var inspect func() = () => { server.SetKeepAlivesEnabled(false) }
 go inspect()
}
func main() { safeLogger(); safeServer() }
