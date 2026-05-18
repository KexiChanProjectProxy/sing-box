package clashapi

import (
	"net/http"

	"github.com/sagernet/sing-box/log"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

func upgradeRouter(server *Server) http.Handler {
	r := chi.NewRouter()
	r.Post("/ui", updateExternalUI(server))
	return r
}

func updateExternalUI(server *Server) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if server.externalUI == "" {
			render.Status(r, http.StatusNotFound)
			render.JSON(w, r, newError("external UI not enabled"))
			return
		}
		server.logger.InfoEvent("clashapi.ui.upgrade", "upgrading external UI")
		err := server.downloadExternalUI()
		if err != nil {
			server.logger.ErrorEvent("clashapi.ui.upgrade.error", "upgrade external ui", log.Err(err))
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, newError(err.Error()))
			return
		}
		server.logger.InfoEvent("clashapi.ui.updated", "updated external UI")
		render.JSON(w, r, render.M{"status": "ok"})
	}
}
