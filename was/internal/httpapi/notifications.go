package httpapi

import (
	"github.com/gin-gonic/gin"
)

func (a *API) notificationRoutes(r *gin.Engine) {
	v := r.Group("/api/v1")
	v.Use(a.require(false))
	v.GET("/notifications", func(c *gin.Context) {
		p, ok := page(c)
		if !ok {
			return
		}
		value, e := a.Notifications.List(c.Request.Context(), principal(c).User.ID, p)
		respond(c, 200, value, e)
	})
	v.POST("/notifications/:id/read", func(c *gin.Context) {
		e := a.Notifications.Read(c.Request.Context(), principal(c).User.ID, c.Param("id"))
		respond(c, 204, struct{}{}, e)
	})
}
