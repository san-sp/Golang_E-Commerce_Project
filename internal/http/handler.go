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

func (h *Handler) PaymentWebhook(c *gin.Context) {
	var request PaymentWebhookRequest

	err := c.ShouldBindJSON(&request)
	if err != nil {
		writeError(
			c,
			http.StatusBadRequest,
			"INVALID_REQUEST",
			"invalid request",
		)
		return
	}

	paymentID, err := uuid.Parse(request.PaymentID)
	if err != nil {
		writeError(
			c,
			http.StatusBadRequest,
			"INVALID_PAYMENT_ID",
			"invalid payment_id",
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
			writeMessage(
				c,
				http.StatusOK,
				"payment webhook already processed",
			)
			return
		}

		if errors.Is(err, payment.ErrReservationExpired) {
			writeError(
				c,
				http.StatusConflict,
				"RESERVATION_EXPIRED",
				"reservation expired",
			)
			return
		}

		writeError(
			c,
			http.StatusInternalServerError,
			"INTERNAL_ERROR",
			"internal server error",
		)
		return
	}

	writeMessage(
		c,
		http.StatusOK,
		"payment webhook processed",
	)
}
