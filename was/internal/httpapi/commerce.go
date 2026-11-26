package httpapi

import (
	"strings"

	"ecommerce.local/base/internal/modules/orders"
	"ecommerce.local/base/internal/modules/users"

	"github.com/gin-gonic/gin"
)

func (a *API) commerce(r *gin.Engine) {
	v := r.Group("/api/v1")
	v.Use(a.require(false), commerceTrace)
	v.GET("/cart", func(c *gin.Context) {
		value, e := a.Cart.Get(c.Request.Context(), principal(c).User.ID)
		respond(c, 200, value, e)
	})
	v.DELETE("/cart", func(c *gin.Context) {
		_, e := a.Cart.Mutate(c.Request.Context(), principal(c).User.ID, "clear", "", "", 0)
		respond(c, 204, struct{}{}, e)
	})
	v.POST("/cart/items", func(c *gin.Context) {
		in, ok := bind[struct {
			ProductID string `json:"product_id"`
			Quantity  int64  `json:"quantity"`
		}](c, "product_id", "quantity")
		if !ok {
			return
		}
		value, e := a.Cart.Mutate(c.Request.Context(), principal(c).User.ID, "add", "", in.ProductID, in.Quantity)
		respond(c, 201, value, e)
	})
	v.PUT("/cart/items/:id", func(c *gin.Context) {
		in, ok := bind[struct {
			Quantity int64 `json:"quantity"`
		}](c, "quantity")
		if !ok {
			return
		}
		value, e := a.Cart.Mutate(c.Request.Context(), principal(c).User.ID, "update", c.Param("id"), "", in.Quantity)
		respond(c, 200, value, e)
	})
	v.DELETE("/cart/items/:id", func(c *gin.Context) {
		_, e := a.Cart.Mutate(c.Request.Context(), principal(c).User.ID, "delete", c.Param("id"), "", 0)
		respond(c, 204, struct{}{}, e)
	})
	v.POST("/orders", func(c *gin.Context) {
		in, ok := bind[orders.CheckoutInput](c, "address_id", "cart_version", "quote_hash", "payment_method")
		if !ok {
			return
		}
		value, e := a.Orders.Checkout(c.Request.Context(), principal(c).User.ID, c.GetHeader("Idempotency-Key"), in, c.GetString("request_id"))
		c.Set("order_id", value.ID)
		respond(c, 201, value, e)
	})
	v.GET("/orders", func(c *gin.Context) {
		p, ok := page(c)
		if !ok {
			return
		}
		value, e := a.Orders.List(c.Request.Context(), principal(c).User.ID, false, p)
		respond(c, 200, value, e)
	})
	v.GET("/orders/:id", func(c *gin.Context) {
		value, e := a.Orders.Get(c.Request.Context(), principal(c).User.ID, c.Param("id"), false)
		respond(c, 200, value, e)
	})
	v.POST("/orders/:id/cancel", func(c *gin.Context) {
		value, e := a.Orders.Transition(c.Request.Context(), principal(c).User.ID, c.Param("id"), c.GetHeader("Idempotency-Key"), "Cancelled", "Customer cancellation", c.GetString("request_id"), false)
		respond(c, 200, value, e)
	})
	admin := r.Group("/api/v1/admin")
	admin.Use(a.require(true), commerceTrace)
	admin.GET("/orders", func(c *gin.Context) {
		p, ok := page(c)
		if !ok {
			return
		}
		value, e := a.Orders.List(c.Request.Context(), principal(c).User.ID, true, p)
		respond(c, 200, value, e)
	})
	admin.GET("/orders/:id", func(c *gin.Context) {
		value, e := a.Orders.Get(c.Request.Context(), principal(c).User.ID, c.Param("id"), true)
		respond(c, 200, value, e)
	})
	admin.PATCH("/orders/:id/status", func(c *gin.Context) {
		in, ok := bind[struct {
			Status string `json:"status"`
			Reason string `json:"reason"`
		}](c, "status")
		if !ok {
			return
		}
		value, e := a.Orders.Transition(c.Request.Context(), principal(c).User.ID, c.Param("id"), c.GetHeader("Idempotency-Key"), in.Status, in.Reason, c.GetString("request_id"), true)
		respond(c, 200, value, e)
	})
	if a.Orders.MockEnabled {
		v.POST("/orders/:id/payment-attempts", func(c *gin.Context) {
			value, e := a.Orders.BeginAttempt(c.Request.Context(), principal(c).User.ID, c.Param("id"), c.GetHeader("Idempotency-Key"))
			c.Set("attempt_id", value.ID)
			respond(c, 201, value, e)
		})
		v.POST("/payment-attempts/:id/simulate", func(c *gin.Context) {
			in, ok := bind[struct {
				Result      string `json:"result"`
				FailureCode string `json:"failure_code"`
			}](c, "result")
			if !ok {
				return
			}
			value, e := a.Orders.Simulate(c.Request.Context(), principal(c).User.ID, c.Param("id"), c.GetHeader("Idempotency-Key"), in.Result, in.FailureCode, c.GetString("request_id"))
			c.Set("order_id", value.ID)
			respond(c, 200, value, e)
		})
	}
}

func commerceTrace(c *gin.Context) {
	id := c.Param("id")
	if users.ValidID(id) {
		if strings.Contains(c.FullPath(), "/payment-attempts/:id/") {
			c.Set("attempt_id", id)
		} else if strings.Contains(c.FullPath(), "/orders/:id") {
			c.Set("order_id", id)
		}
	}
	c.Next()
}
