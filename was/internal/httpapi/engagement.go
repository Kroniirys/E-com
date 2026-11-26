package httpapi

import (
	"ecommerce.local/base/internal/modules/admin"
	"ecommerce.local/base/internal/modules/coupons"
	"ecommerce.local/base/internal/modules/reviews"
	"github.com/gin-gonic/gin"
)

func (a *API) engagement(r *gin.Engine) {
	v := r.Group("/api/v1")
	v.Use(a.require(false))
	v.POST("/coupons/validate", func(c *gin.Context) {
		in, ok := bind[struct {
			Code        string `json:"code"`
			CartVersion int64  `json:"cart_version"`
		}](c, "code", "cart_version")
		if !ok {
			return
		}
		value, e := a.Orders.PreviewCoupon(c.Request.Context(), principal(c).User.ID, in.Code, in.CartVersion)
		respond(c, 200, value, e)
	})
	v.GET("/products/:id/review-eligibility", func(c *gin.Context) {
		value, e := a.Reviews.Eligibility(c.Request.Context(), principal(c).User.ID, c.Param("id"))
		respond(c, 200, value, e)
	})
	v.POST("/products/:id/reviews", func(c *gin.Context) {
		in, ok := bind[reviews.Input](c, "rating", "content")
		if !ok {
			return
		}
		value, e := a.Reviews.Create(c.Request.Context(), principal(c).User.ID, c.Param("id"), in)
		respond(c, 201, value, e)
	})
	v.GET("/me/reviews", func(c *gin.Context) {
		p, ok := page(c)
		if !ok {
			return
		}
		value, e := a.Reviews.Mine(c.Request.Context(), principal(c).User.ID, p)
		respond(c, 200, value, e)
	})
	v.DELETE("/reviews/:id", func(c *gin.Context) {
		e := a.Reviews.Delete(c.Request.Context(), principal(c).User.ID, c.Param("id"))
		respond(c, 204, struct{}{}, e)
	})
	r.GET("/api/v1/products/:id/reviews", func(c *gin.Context) {
		p, ok := page(c)
		if !ok {
			return
		}
		value, e := a.Reviews.Public(c.Request.Context(), c.Param("id"), p)
		respond(c, 200, value, e)
	})
	root := r.Group("/api/v1/admin")
	root.Use(a.require(true))
	root.GET("/coupons", func(c *gin.Context) {
		p, ok := page(c)
		if !ok {
			return
		}
		value, e := a.Coupons.List(c.Request.Context(), p)
		respond(c, 200, value, e)
	})
	root.GET("/coupons/:id", func(c *gin.Context) {
		value, e := a.Coupons.Get(c.Request.Context(), c.Param("id"))
		respond(c, 200, value, e)
	})
	root.POST("/coupons", func(c *gin.Context) { a.saveCoupon(c, "") })
	root.PUT("/coupons/:id", func(c *gin.Context) { a.saveCoupon(c, c.Param("id")) })
	root.DELETE("/coupons/:id", func(c *gin.Context) {
		e := a.Coupons.Archive(c.Request.Context(), principal(c).User.ID, c.Param("id"))
		respond(c, 204, struct{}{}, e)
	})
	root.GET("/reviews", func(c *gin.Context) {
		p, ok := page(c)
		if !ok {
			return
		}
		value, e := a.Reviews.AdminList(c.Request.Context(), p, c.Query("visibility"))
		respond(c, 200, value, e)
	})
	root.PATCH("/reviews/:id/moderation", func(c *gin.Context) {
		in, ok := bind[struct {
			Visibility string `json:"visibility"`
			Reason     string `json:"reason"`
		}](c, "visibility", "reason")
		if !ok {
			return
		}
		value, e := a.Reviews.Moderate(c.Request.Context(), principal(c).User.ID, c.Param("id"), in.Visibility, in.Reason)
		respond(c, 200, value, e)
	})
	root.GET("/dashboard", func(c *gin.Context) {
		span, e := admin.ParseRange(c.Query("from"), c.Query("to"), c.Query("timezone"))
		if e != nil {
			fail(c, e)
			return
		}
		value, e := a.Dashboard.Get(c.Request.Context(), span)
		respond(c, 200, value, e)
	})
}
func (a *API) saveCoupon(c *gin.Context, id string) {
	in, ok := bind[coupons.Input](c, "code", "kind", "value", "minimum_minor", "expires_at", "usage_limit", "enabled")
	if !ok {
		return
	}
	value, e := a.Coupons.Save(c.Request.Context(), principal(c).User.ID, id, in)
	status := 200
	if id == "" {
		status = 201
	}
	respond(c, status, value, e)
}
