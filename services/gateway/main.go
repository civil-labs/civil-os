package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// Configuration Variables
var (
	// The Internal address of the Martin/Nginx Service (via Service Connect)
	// Example: "http://tile-server:80"
	TileServerURL = getEnv("TILE_SERVER_URL", "http://tile-server:8080")

	// The Internal address of the IDP
	// Example: "http://idp:5556"
	AuthServerURL = getEnv("IDP_URL", "http://idp:5556")

	// The Shared Secret to verify JWT signatures (Must match Auth Server)
	JwtSecret = []byte(getEnv("JWT_SECRET", "super-duper-secret-dev-key"))
)

func main() {
	r := gin.Default()

	// --- 1. The "Authorize" Endpoint (Login) ---
	// Called by Next.js Server Side.
	// Takes {username, password}, returns {token}
	r.POST("/api/login", handleLoginProxy)

	// --- 2. The "Tile" Endpoint (Protected) ---
	// Called by Browser (MapLibre).
	// Validates Token, then streams data from Nginx.
	r.GET("/tiles/*path", authMiddleware(), handleTileProxy)

	// Health check for Load Balancer
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	port := getEnv("PORT", "8080")
	log.Printf("Civil Bridge starting on port %s...", port)
	r.Run(":" + port)
}

// ---------------------------------------------------------
// HANDLER 1: Login Proxy (Forwarding Credentials)
// ---------------------------------------------------------
func handleLoginProxy(c *gin.Context) {
	// 1. Read credentials from Next.js
	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.BindJSON(&creds); err != nil {
		c.JSON(400, gin.H{"error": "Invalid JSON"})
		return
	}

	// 2. Prepare request to Internal Auth Server
	authPayload, _ := json.Marshal(creds)
	resp, err := http.Post(
		AuthServerURL+"/token", // Assuming your auth server has this endpoint
		"application/json",
		bytes.NewBuffer(authPayload),
	)
	if err != nil {
		c.JSON(502, gin.H{"error": "Auth server unreachable"})
		return
	}
	defer resp.Body.Close()

	// 3. Forward the Auth Server's response back to Next.js
	// We simply copy the status code and the body (which contains the Token)
	c.Status(resp.StatusCode)
	io.Copy(c.Writer, resp.Body)
}

// ---------------------------------------------------------
// MIDDLEWARE: JWT Validator
// ---------------------------------------------------------
func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Extract Token from Header or Query Param
		// MapLibre often sends tokens in query params: ?token=xyz
		tokenString := c.Query("token")
		
		// Fallback to Authorization Header
		if tokenString == "" {
			authHeader := c.GetHeader("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				tokenString = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		if tokenString == "" {
			c.AbortWithStatusJSON(401, gin.H{"error": "Missing token"})
			return
		}

		// 2. Verify Token Signature
		// This validates that WE (or our trusted Auth Server) signed it.
		// It does NOT call the Auth Server every time (Stateless validation).
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return JwtSecret, nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(403, gin.H{"error": "Invalid or expired token"})
			return
		}

		// 3. (Optional) Extract Claims for Policy Engine
		// claims := token.Claims.(jwt.MapClaims)
		// role := claims["role"]
		// c.Set("user_role", role)

		c.Next()
	}
}

// ---------------------------------------------------------
// HANDLER 2: Tile Proxy (Streaming Reverse Proxy)
// ---------------------------------------------------------
func handleTileProxy(c *gin.Context) {
	remote, err := url.Parse(TileServerURL)
	if err != nil {
		c.JSON(500, gin.H{"error": "Misconfigured upstream"})
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(remote)

	// Customize the Director to rewrite the path
	// Incoming: /tiles/martin/get_mvt...
	// Outgoing: /martin/get_mvt... (or whatever Nginx expects)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		
		// 1. Strip the "/tiles" prefix so Nginx gets the clean path
		// Example: /tiles/rpc/get_mvt -> /rpc/get_mvt
		req.URL.Path = strings.TrimPrefix(req.URL.Path, "/tiles")
		
		// 2. Set Host header so Nginx/Martin knows who we are
		// (Important if Nginx has vhost routing)
		req.Host = remote.Host
	}

	// ServeHTTP streams the response directly.
	// It handles GZIP, flush, and headers automatically.
	proxy.ServeHTTP(c.Writer, c.Request)
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}