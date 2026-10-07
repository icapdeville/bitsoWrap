package handlers

import (
	"net/http"
	"os"
)

// HealthHandler responde si el proceso está vivo, sin token y sin llamar a
// Bitso (lo sondea el panel de servicios cada pocos segundos). Sin la key de
// solo lectura en el entorno, /saldo y /saldos no funcionan: queda degradado.
//
//	GET /health  -> {"status":"ok","credenciales":"ok"}
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	status, creds := "ok", "ok"
	if os.Getenv("BITSO_API_KEY") == "" || os.Getenv("BITSO_API_SECRET") == "" {
		status, creds = "degraded", "faltan"
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": status, "credenciales": creds})
}
