package http

import "github.com/gin-gonic/gin"

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(
	c *gin.Context,
	status int,
	code string,
	message string,
) {
	c.JSON(
		status,
		gin.H{
			"error": ErrorResponse{
				Code:    code,
				Message: message,
			},
		},
	)
}

func writeMessage(
	c *gin.Context,
	status int,
	message string,
) {
	c.JSON(
		status,
		gin.H{
			"message": message,
		},
	)
}
