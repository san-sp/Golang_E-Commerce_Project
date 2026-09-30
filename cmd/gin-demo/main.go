package main

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func contextMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		fmt.Println(
			"Go context:",
			ctx,
		)

		c.Set("user_id", "user-123")

		c.Next()
	}
}

func main() {
	router := gin.New()

	api := router.Group("/api/v1")

	api.Use(contextMiddleware())

	api.GET("/payments", func(c *gin.Context) {
		userID, exists := c.Get("user_id")

		if !exists {
			c.JSON(
				http.StatusInternalServerError,
				gin.H{
					"error": "user ID not found",
				},
			)
			return
		}

		ctx := c.Request.Context()

		c.JSON(
			http.StatusOK,
			gin.H{
				"message": "Payment endpoint reached",
				"user_id": userID,
				"context": fmt.Sprintf("%v", ctx),
			},
		)
	})

	router.Run(":8080")
}
