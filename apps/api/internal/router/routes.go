package router

import "github.com/go-golpher/golpher"

func RegisterRoutes(app *golpher.App, handlers *Handlers) {
	app.GET("/ready", handlers.Ready)
	app.POST("/fraud-score", handlers.FraudScore)
}
