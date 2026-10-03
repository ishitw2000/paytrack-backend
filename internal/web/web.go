package web

import (
	_ "embed"
	"net/http"
)

//go:embed admin.html
var AdminHTML []byte

func ServeAdminDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(AdminHTML)
}
