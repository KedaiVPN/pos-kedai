package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"kasir-api-go/db"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func AuthMiddleware(queries *db.Queries) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			c.Abort()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header format must be Bearer {token}"})
			c.Abort()
			return
		}

		tokenString := parts[1]
		jwtSecret := GetJWTSecret()

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(jwtSecret), nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			c.Abort()
			return
		}

		userIDVal := claims["user_id"]
		c.Set("user_id", userIDVal)
		role, _ := claims["role"].(string)
		c.Set("role", role)

		// Verification: User & Store DB check for auto-logout when store or user is deleted
		// Always check user & store regardless of role, so that deleted store/user triggers 401 logout for everyone
		if queries != nil {
			userIDStr, ok := userIDVal.(string)
			if !ok || userIDStr == "" {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID tidak valid dalam token"})
				c.Abort()
				return
			}

			parsedUserID, err := uuid.Parse(userIDStr)
			if err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID tidak valid"})
				c.Abort()
				return
			}

			user, err := queries.GetUser(c.Request.Context(), pgtype.UUID{Bytes: parsedUserID, Valid: true})
			if err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Akun tidak ditemukan atau telah dihapus. Sesi login berakhir."})
				c.Abort()
				return
			}

			if user.IsActive.Valid && !user.IsActive.Bool {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Akun tidak aktif."})
				c.Abort()
				return
			}
		}

		if storeIDVal, exists := claims["store_id"]; exists {
			c.Set("store_id", storeIDVal)

			if storeIDStr, ok := storeIDVal.(string); ok && storeIDStr != "" {
				parsedStoreID, err := uuid.Parse(storeIDStr)
				if err == nil && queries != nil {
					// 1. Verify store still exists in database
					_, err := queries.GetStore(c.Request.Context(), pgtype.UUID{Bytes: parsedStoreID, Valid: true})
					if err != nil {
						c.JSON(http.StatusUnauthorized, gin.H{"error": "Toko telah dihapus. Sesi login berakhir."})
						c.Abort()
						return
					}

					// 2. Check cashier Pro status expiration
					if role == "kasir" {
						storeStatus, err := queries.GetStoreProStatus(c.Request.Context(), pgtype.UUID{Bytes: parsedStoreID, Valid: true})
						if err != nil || !storeStatus.ProExpiresAt.Valid || storeStatus.ProExpiresAt.Time.Before(time.Now()) {
							c.JSON(http.StatusUnauthorized, gin.H{"error": "Masa aktif Pro toko telah berakhir. Sesi login karyawan dikunci."})
							c.Abort()
							return
						}
					}

					// 3. Check if store is blocked (for owner & cashier)
					if role == "owner" || role == "kasir" {
						storeDetail, err := queries.GetStore(c.Request.Context(), pgtype.UUID{Bytes: parsedStoreID, Valid: true})
						if err == nil && storeDetail.IsBlocked {
							c.JSON(http.StatusForbidden, gin.H{
								"error": gin.H{
									"code":    "STORE_BLOCKED",
									"message": "Toko ini sedang dikunci. Silakan hubungi admin untuk membuka kunci.",
								},
							})
							c.Abort()
							return
						}
					}
				}
			}
		}

		c.Next()
	}
}
