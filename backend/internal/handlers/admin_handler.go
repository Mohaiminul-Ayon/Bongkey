package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"bongkey/backend/internal/config"
	"bongkey/backend/internal/database"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AdminHandler struct {
	Config config.Config
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h AdminHandler) Login(c *gin.Context) {
	if !database.Ready() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database is not connected"})
		return
	}

	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email and password are required"})
		return
	}

	var id int
	var name, email, passwordHash string
	err := database.DB.QueryRow(`SELECT id, name, email, password_hash FROM admins WHERE email = $1`, req.Email).Scan(&id, &name, &email, &passwordHash)
	if err == sql.ErrNoRows || bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "login failed"})
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"admin_id": id,
		"email":    email,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	})
	tokenString, err := token.SignedString([]byte(h.Config.JWTSecret))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create session"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": tokenString,
		"admin": gin.H{"id": id, "name": name, "email": email},
	})
}

func (h AdminHandler) Me(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"authenticated": true})
}
