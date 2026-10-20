package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           int
}

func CORS(cfg CORSConfig) gin.HandlerFunc {
	validateCORS(cfg)

	origins := makeSet(cfg.AllowedOrigins)

	methods := strings.Join(cfg.AllowedMethods, ", ")
	headers := strings.Join(cfg.AllowedHeaders, ", ")
	exposed := strings.Join(cfg.ExposedHeaders, ", ")

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		if origin == "" {
			c.Next()
			return
		}

		_, allowed := origins[origin]
		_, wildcard := origins["*"]
		c.Writer.Header().Add("Vary", "Origin")
		if !allowed && !wildcard {
			if c.Request.Method == http.MethodOptions {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}

			c.Next()
			return
		}

		h := c.Writer.Header()

		if wildcard {
			h.Set("Access-Control-Allow-Origin", "*")
		} else {
			h.Set("Access-Control-Allow-Origin", origin)
		}

		if methods != "" {
			h.Set("Access-Control-Allow-Methods", methods)
		}

		if headers != "" {
			h.Set("Access-Control-Allow-Headers", headers)
		}

		if exposed != "" {
			h.Set("Access-Control-Expose-Headers", exposed)
		}

		if cfg.AllowCredentials {
			h.Set("Access-Control-Allow-Credentials", "true")
		}

		if cfg.MaxAge > 0 {
			h.Set(
				"Access-Control-Max-Age",
				strconv.Itoa(cfg.MaxAge),
			)
		}

		if c.Request.Method == http.MethodOptions && c.GetHeader("Access-Control-Request-Method") != "" {
			h.Add("Vary", "Access-Control-Request-Method")
			h.Add("Vary", "Access-Control-Request-Headers")
			if _, ok := makeSet(cfg.AllowedMethods)[c.GetHeader("Access-Control-Request-Method")]; !ok {
				c.AbortWithStatus(403)
				return
			}
			permitted := makeSet(nil)
			for _, v := range cfg.AllowedHeaders {
				permitted[strings.ToLower(v)] = struct{}{}
			}
			for _, v := range strings.Split(c.GetHeader("Access-Control-Request-Headers"), ",") {
				if v = strings.TrimSpace(v); v != "" {
					if _, ok := permitted[strings.ToLower(v)]; !ok {
						c.AbortWithStatus(403)
						return
					}
				}
			}
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func validateCORS(cfg CORSConfig) {
	if len(cfg.AllowedOrigins) == 0 {
		panic("middleware: CORS requires at least one allowed origin")
	}

	if cfg.AllowCredentials {
		for _, origin := range cfg.AllowedOrigins {
			if origin == "*" {
				panic(
					"middleware: wildcard origin cannot be used with credentials",
				)
			}
		}
	}
}

func makeSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))

	for _, value := range values {
		set[value] = struct{}{}
	}

	return set
}
