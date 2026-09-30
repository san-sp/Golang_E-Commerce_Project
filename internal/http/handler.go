package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/san-sp/Golang_E-Commerce_Project/internal/payment"
)

type Handler struct {
	paymentService *payment.Service
}

func NewHandler(
	paymentService *payment.Service,
) *Handler {
	return &Handler{
		paymentService: paymentService,
	}
}

func (h *Handler) PaymentWebhook(c *gin.Context) {
	var webhook payment.PaymentWebhook

	err := c.ShouldBindJSON(&webhook)
	if err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"error": "invalid JSON",
			},
		)
		return
	}

	_, err = h.paymentService.ProcessPaymentWebhook(
		c.Request.Context(),
		webhook,
	)
	if err != nil {
		if errors.Is(err, payment.ErrDuplicateWebhook) {
			c.JSON(
				http.StatusOK,
				gin.H{
					"message": "payment webhook already processed",
				},
			)
			return
		}

		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"error": err.Error(),
			},
		)
		return
	}

	c.JSON(
		http.StatusOK,
		gin.H{
			"message": "payment webhook processed",
		},
	)
}
