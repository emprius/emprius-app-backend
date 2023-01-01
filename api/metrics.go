package api

import chiprometheus "github.com/766b/chi-prometheus"

// EnablePrometheusMetrics enables go-chi prometheus metrics under specified ID.
// If ID empty, the default "gochi_http" is used.
func (a *API) EnablePrometheusMetrics(prometheusID string) {
	// Prometheus handler
	if prometheusID == "" {
		prometheusID = "gochi_http"
	}
	a.Router.Use(chiprometheus.NewMiddleware(prometheusID))
}
