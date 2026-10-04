package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/san-sp/Golang_E-Commerce_Project/internal/payment"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/reservation"
)

type Handler struct {
	paymentService     *payment.Service
	reservationService *reservation.Service
}

func NewHandler(
	paymentService *payment.Service,
	reservationService *reservation.Service,
) *Handler {
	return &Handler{
		paymentService:     paymentService,
		reservationService: reservationService,
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

func (h *Handler) CreatePayment(c *gin.Context) {
	var request CreatePaymentRequest

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

	reservationID, err := uuid.Parse(request.ReservationID)
	if err != nil {
		writeError(
			c,
			http.StatusBadRequest,
			"INVALID_RESERVATION_ID",
			"invalid reservation_id",
		)
		return
	}

	payment, err := h.paymentService.CreatePayment(
		c.Request.Context(),
		pgtype.UUID{
			Bytes: reservationID,
			Valid: true,
		},
		request.Provider,
		request.Amount,
		request.Currency,
	)
	if err != nil {
		writeError(
			c,
			http.StatusInternalServerError,
			"PAYMENT_CREATION_FAILED",
			"payment creation failed",
		)
		return
	}

	c.JSON(
		http.StatusCreated,
		gin.H{
			"id":                  payment.ID,
			"status":              payment.Status,
			"reservation_id":      payment.ReservationID,
			"provider":            payment.Provider,
			"provider_payment_id": payment.ProviderPaymentID,
			"amount":              payment.Amount,
			"currency":            payment.Currency,
		},
	)
}

func (h *Handler) CreateReservation(c *gin.Context) {
	var request CreateReservationRequest

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

	variantID, err := uuid.Parse(request.VariantID)
	if err != nil {
		writeError(
			c,
			http.StatusBadRequest,
			"INVALID_VARIANT_ID",
			"invalid variant_id",
		)
		return
	}

	createdReservation, err := h.reservationService.CreateReservation(
		c.Request.Context(),
		pgtype.UUID{
			Bytes: variantID,
			Valid: true,
		},
		request.Quantity,
	)
	if err != nil {
		if errors.Is(err, reservation.ErrInsufficientStock) {
			writeError(
				c,
				http.StatusConflict,
				"INSUFFICIENT_STOCK",
				"insufficient stock",
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

	c.JSON(
		http.StatusCreated,
		gin.H{
			"id":         createdReservation.ID,
			"variant_id": createdReservation.VariantID,
			"quantity":   createdReservation.Quantity,
			"status":     createdReservation.Status,
			"expires_at": createdReservation.ExpiresAt,
		},
	)
}
