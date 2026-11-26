package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"ecommerce.local/base/internal/modules/auth"
	"ecommerce.local/base/internal/modules/catalog"
	"ecommerce.local/base/internal/modules/inventory"
	"ecommerce.local/base/internal/modules/users"
	"ecommerce.local/base/internal/platform/problem"
	"github.com/gin-gonic/gin"
)

type API struct {
	Auth         *auth.Service
	Users        *users.Service
	Catalog      *catalog.Service
	Inventory    *inventory.Service
	Origins      []string
	SecureCookie bool
}

func fail(c *gin.Context, err error) {
	p := problem.Normalize(err)
	if p.RetryAfter > 0 {
		c.Header("Retry-After", strconv.Itoa(p.RetryAfter))
	}
	c.AbortWithStatusJSON(p.Status, gin.H{"error": gin.H{"code": p.Code, "message": p.Message, "request_id": c.GetString("request_id")}})
}
func respond[T any](c *gin.Context, status int, value T, err error) {
	if err != nil {
		fail(c, err)
		return
	}
	if status == 204 {
		c.Status(204)
		return
	}
	c.JSON(status, gin.H{"data": value})
}
func bind[T any](c *gin.Context, required ...string) (T, bool) {
	var input T
	media, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || media != "application/json" {
		fail(c, problem.Invalid("Content-Type must be application/json"))
		return input, false
	}
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		fail(c, problem.Invalid("Request body is too large or invalid"))
		return input, false
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&input); err != nil {
		fail(c, problem.Invalid("Request JSON contains invalid or unsupported fields"))
		return input, false
	}
	if err = decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		fail(c, problem.Invalid("Only one JSON object is allowed"))
		return input, false
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(raw, &fields); err != nil || fields == nil {
		fail(c, problem.Invalid("A JSON object is required"))
		return input, false
	}
	for _, name := range required {
		value, ok := fields[name]
		if !ok || string(value) == "null" {
			fail(c, problem.Invalid("Required field is missing: "+name))
			return input, false
		}
	}
	return input, true
}
func page(c *gin.Context) (users.Page, bool) {
	result := users.Page{Number: 1, Size: 20, Query: strings.TrimSpace(c.Query("q")), Sort: c.Query("sort"), CategoryID: c.Query("category_id")}
	if v := c.Query("page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 100000 {
			fail(c, problem.Invalid("page must be between 1 and 100000"))
			return result, false
		}
		result.Number = n
	}
	if v := c.Query("page_size"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 100 {
			fail(c, problem.Invalid("page_size must be between 1 and 100"))
			return result, false
		}
		result.Size = n
	}
	if utf8.RuneCountInString(result.Query) > 200 {
		fail(c, problem.Invalid("Search must not exceed 200 characters"))
		return result, false
	}
	return result, true
}
func (a *API) origin(c *gin.Context) bool {
	for _, origin := range a.Origins {
		if c.GetHeader("Origin") == origin {
			return true
		}
	}
	fail(c, problem.New(403, "ORIGIN_REQUIRED", "A trusted Origin is required for cookie authentication"))
	return false
}
func (a *API) cookie(c *gin.Context, raw string, expires time.Time) {
	maxAge := int(time.Until(expires).Seconds())
	if raw == "" {
		maxAge = -1
	}
	http.SetCookie(c.Writer, &http.Cookie{Name: "refresh_token", Value: raw, Path: "/api/v1/auth", HttpOnly: true, Secure: a.SecureCookie, SameSite: http.SameSiteLaxMode, Expires: expires, MaxAge: maxAge})
}
func (a *API) rate(c *gin.Context, scope string, maximum int) bool {
	retry, err := a.Auth.Rate(c.Request.Context(), scope, c.ClientIP(), maximum)
	if err != nil {
		if retry > 0 {
			c.Header("Retry-After", strconv.Itoa(retry))
		}
		fail(c, err)
		return false
	}
	return true
}
func principal(c *gin.Context) auth.Principal { return c.MustGet("principal").(auth.Principal) }
func (a *API) require(admin bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			fail(c, problem.Unauthorized())
			return
		}
		p, err := a.Auth.Authenticate(c.Request.Context(), strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			fail(c, err)
			return
		}
		if admin && p.User.Role != "Admin" {
			fail(c, problem.Forbidden())
			return
		}
		c.Set("principal", p)
		c.Next()
	}
}
func (a *API) Register(r *gin.Engine) {
	v := r.Group("/api/v1")
	v.POST("/auth/register", func(c *gin.Context) {
		if !a.origin(c) || !a.rate(c, "register-ip", 5) {
			return
		}
		in, ok := bind[struct {
			Email    string `json:"email"`
			Name     string `json:"name"`
			Password string `json:"password"`
		}](c, "email", "name", "password")
		if !ok {
			return
		}
		user, err := a.Auth.Register(c.Request.Context(), users.Input{Email: in.Email, Name: in.Name, Password: in.Password})
		respond(c, 201, user, err)
	})
	v.POST("/auth/login", func(c *gin.Context) {
		if !a.origin(c) || !a.rate(c, "login-ip", 5) {
			return
		}
		in, ok := bind[struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}](c, "email", "password")
		if !ok {
			return
		}
		if len(in.Email) > 254 || len(in.Password) > 72 || in.Email == "" || in.Password == "" {
			fail(c, problem.Invalid("Email and password are required within their size limits"))
			return
		}
		tokens, err := a.Auth.Login(c.Request.Context(), in.Email, in.Password)
		if err != nil {
			fail(c, err)
			return
		}
		a.cookie(c, tokens.Refresh, tokens.RefreshExpires)
		respond(c, 200, tokens, nil)
	})
	v.POST("/auth/refresh", func(c *gin.Context) {
		if !a.origin(c) || !a.rate(c, "refresh-ip", 20) {
			return
		}
		raw, _ := c.Cookie("refresh_token")
		tokens, err := a.Auth.Refresh(c.Request.Context(), raw)
		if err != nil {
			if problem.Normalize(err).Status == 401 {
				a.cookie(c, "", time.Unix(1, 0))
			}
			fail(c, err)
			return
		}
		a.cookie(c, tokens.Refresh, tokens.RefreshExpires)
		respond(c, 200, tokens, nil)
	})
	v.POST("/auth/logout", func(c *gin.Context) {
		if !a.origin(c) {
			return
		}
		raw, _ := c.Cookie("refresh_token")
		err := a.Auth.Logout(c.Request.Context(), raw)
		if err == nil {
			a.cookie(c, "", time.Unix(1, 0))
		}
		respond(c, 204, struct{}{}, err)
	})
	protected := v.Group("")
	protected.Use(a.require(false))
	protected.GET("/auth/me", func(c *gin.Context) { respond(c, 200, principal(c).User, nil) })
	protected.POST("/auth/change-password", func(c *gin.Context) {
		in, ok := bind[struct {
			Current string `json:"current_password"`
			Next    string `json:"new_password"`
		}](c, "current_password", "new_password")
		if !ok {
			return
		}
		if len(in.Current) > 72 {
			fail(c, problem.Invalid("Current password exceeds the limit"))
			return
		}
		err := a.Auth.ChangePassword(c.Request.Context(), principal(c), in.Current, in.Next)
		if err == nil {
			a.cookie(c, "", time.Unix(1, 0))
		}
		respond(c, 204, struct{}{}, err)
	})
	protected.PATCH("/me", func(c *gin.Context) {
		in, ok := bind[struct {
			Name string `json:"name"`
		}](c, "name")
		if !ok {
			return
		}
		value, err := a.Users.Profile(c.Request.Context(), principal(c).User.ID, in.Name)
		respond(c, 200, value, err)
	})
	protected.GET("/me/addresses", func(c *gin.Context) {
		p, ok := page(c)
		if !ok {
			return
		}
		value, err := a.Users.Addresses(c.Request.Context(), principal(c).User.ID, p)
		respond(c, 200, value, err)
	})
	protected.POST("/me/addresses", func(c *gin.Context) { a.saveAddress(c, "") })
	protected.PUT("/me/addresses/:id", func(c *gin.Context) { a.saveAddress(c, c.Param("id")) })
	protected.DELETE("/me/addresses/:id", func(c *gin.Context) {
		err := a.Users.DeleteAddress(c.Request.Context(), principal(c).User.ID, c.Param("id"))
		respond(c, 204, struct{}{}, err)
	})
	v.GET("/products", func(c *gin.Context) { a.products(c, false) })
	v.GET("/products/:id", func(c *gin.Context) {
		value, err := a.Catalog.Product(c.Request.Context(), c.Param("id"), false)
		respond(c, 200, value, err)
	})
	v.GET("/categories", a.categories)
	v.GET("/categories/:id", func(c *gin.Context) {
		value, err := a.Catalog.Category(c.Request.Context(), c.Param("id"))
		respond(c, 200, value, err)
	})
	admin := v.Group("/admin")
	admin.Use(a.require(true))
	admin.GET("/users", func(c *gin.Context) {
		p, ok := page(c)
		if !ok {
			return
		}
		value, err := a.Users.List(c.Request.Context(), p)
		respond(c, 200, value, err)
	})
	admin.GET("/users/:id", func(c *gin.Context) {
		value, err := a.Users.AdminGet(c.Request.Context(), c.Param("id"))
		respond(c, 200, value, err)
	})
	admin.POST("/users", func(c *gin.Context) {
		in, ok := bind[users.Input](c, "email", "name", "password", "role", "status")
		if !ok {
			return
		}
		value, err := a.Users.AdminCreate(c.Request.Context(), principal(c).User.ID, in)
		respond(c, 201, value, err)
	})
	admin.PUT("/users/:id", func(c *gin.Context) {
		in, ok := bind[struct {
			Email  string `json:"email"`
			Name   string `json:"name"`
			Role   string `json:"role"`
			Status string `json:"status"`
		}](c, "email", "name", "role", "status")
		if !ok {
			return
		}
		value, err := a.Users.AdminUpdate(c.Request.Context(), principal(c).User.ID, c.Param("id"), users.Input{Email: in.Email, Name: in.Name, Role: in.Role, Status: in.Status}, false)
		respond(c, 200, value, err)
	})
	admin.DELETE("/users/:id", func(c *gin.Context) {
		_, err := a.Users.AdminUpdate(c.Request.Context(), principal(c).User.ID, c.Param("id"), users.Input{}, true)
		respond(c, 204, struct{}{}, err)
	})
	admin.GET("/products", func(c *gin.Context) { a.products(c, true) })
	admin.GET("/products/:id", func(c *gin.Context) {
		value, err := a.Catalog.Product(c.Request.Context(), c.Param("id"), true)
		respond(c, 200, value, err)
	})
	admin.POST("/products", func(c *gin.Context) { a.saveProduct(c, "") })
	admin.PUT("/products/:id", func(c *gin.Context) { a.saveProduct(c, c.Param("id")) })
	admin.DELETE("/products/:id", func(c *gin.Context) {
		respond(c, 204, struct{}{}, a.Catalog.DeleteProduct(c.Request.Context(), c.Param("id")))
	})
	admin.GET("/categories", a.categories)
	admin.POST("/categories", func(c *gin.Context) { a.saveCategory(c, "") })
	admin.PUT("/categories/:id", func(c *gin.Context) { a.saveCategory(c, c.Param("id")) })
	admin.DELETE("/categories/:id", func(c *gin.Context) {
		respond(c, 204, struct{}{}, a.Catalog.DeleteCategory(c.Request.Context(), c.Param("id")))
	})
	admin.GET("/inventory", func(c *gin.Context) {
		p, ok := page(c)
		if !ok {
			return
		}
		value, err := a.Inventory.List(c.Request.Context(), p)
		respond(c, 200, value, err)
	})
	admin.GET("/inventory/:id/history", func(c *gin.Context) {
		p, ok := page(c)
		if !ok {
			return
		}
		value, err := a.Inventory.History(c.Request.Context(), c.Param("id"), p)
		respond(c, 200, value, err)
	})
	admin.POST("/inventory/adjustments", func(c *gin.Context) {
		in, ok := bind[inventory.Adjustment](c, "product_id", "delta", "reason")
		if !ok {
			return
		}
		value, err := a.Inventory.Adjust(c.Request.Context(), principal(c).User.ID, c.GetHeader("Idempotency-Key"), in)
		respond(c, 200, value, err)
	})
}
func (a *API) saveAddress(c *gin.Context, id string) {
	in, ok := bind[users.AddressInput](c, "recipient", "phone", "address", "is_default")
	if !ok {
		return
	}
	value, err := a.Users.SaveAddress(c.Request.Context(), principal(c).User.ID, id, in)
	status := 200
	if id == "" {
		status = 201
	}
	respond(c, status, value, err)
}
func (a *API) products(c *gin.Context, admin bool) {
	p, ok := page(c)
	if !ok {
		return
	}
	value, err := a.Catalog.Products(c.Request.Context(), p, admin)
	respond(c, 200, value, err)
}
func (a *API) categories(c *gin.Context) {
	p, ok := page(c)
	if !ok {
		return
	}
	value, err := a.Catalog.Categories(c.Request.Context(), p)
	respond(c, 200, value, err)
}
func (a *API) saveProduct(c *gin.Context, id string) {
	in, ok := bind[catalog.ProductInput](c, "sku", "name", "description", "price_minor", "original_price_minor", "status", "images", "category_ids")
	if !ok {
		return
	}
	value, err := a.Catalog.SaveProduct(c.Request.Context(), id, in)
	status := 200
	if id == "" {
		status = 201
	}
	respond(c, status, value, err)
}
func (a *API) saveCategory(c *gin.Context, id string) {
	in, ok := bind[catalog.CategoryInput](c, "name", "slug")
	if !ok {
		return
	}
	value, err := a.Catalog.SaveCategory(c.Request.Context(), id, in)
	status := 200
	if id == "" {
		status = 201
	}
	respond(c, status, value, err)
}
