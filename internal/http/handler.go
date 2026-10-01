package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

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

func writeInternalServerError(c *gin.Context) {
	c.JSON(
		http.StatusInternalServerError,
		gin.H{
			"error": "internal server error",
		},
	)
}

func (h *Handler) PaymentWebhook(c *gin.Context) {
	var request PaymentWebhookRequest

	err := c.ShouldBindJSON(&request)
	if err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"error": "invalid request",
			},
		)
		return
	}

	paymentID, err := uuid.Parse(request.PaymentID)
	if err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"error": "invalid payment_id",
			},
		)
		return
	}

	webhook := payment.PaymentWebhook{
		EventID:   request.EventID,
		EventType: request.EventType,
		PaymentID: pgtype.UUID{
			Bytes: paymentID,
			Valid: true,
		},
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

		writeInternalServerError(c)
		return
	}

	c.JSON(
		http.StatusOK,
		gin.H{
			"message": "payment webhook processed",
		},
	)
}
