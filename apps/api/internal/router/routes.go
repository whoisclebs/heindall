package router

import (
	"net/http"

	"github.com/go-golpher/golpher"
)

func RegisterRoutes(app *golpher.App, handlers *Handlers) {
	app.Raw(http.MethodGet, "/ready", handlers.ReadyRaw)
	app.Raw(http.MethodPost, "/fraud-score", handlers.FraudScoreRaw)
}
