package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/san-sp/Golang_E-Commerce_Project/internal/cart"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/payment"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/reservation"
)

type Handler struct {
	paymentService     *payment.Service
	reservationService *reservation.Service
	cartService        *cart.Service
}

func NewHandler(
	paymentService *payment.Service,
	reservationService *reservation.Service,
	cartService *cart.Service,
) *Handler {
	return &Handler{
		paymentService:     paymentService,
		reservationService: reservationService,
		cartService:        cartService,
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

func (h *Handler) CreateCart(c *gin.Context) {
	cart, err := h.cartService.CreateCart(
		c.Request.Context(),
	)
	if err != nil {
		writeError(
			c,
			http.StatusInternalServerError,
			"CART_CREATION_FAILED",
			"cart creation failed",
		)
		return
	}

	c.JSON(
		http.StatusCreated,
		gin.H{
			"id":         cart.ID,
			"status":     cart.Status,
			"created_at": cart.CreatedAt,
			"updated_at": cart.UpdatedAt,
		},
	)
}

func (h *Handler) AddCartItem(c *gin.Context) {
	cartID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(
			c,
			http.StatusBadRequest,
			"INVALID_CART_ID",
			"invalid cart id",
		)
		return
	}

	var request AddCartItemRequest

	if err := c.ShouldBindJSON(&request); err != nil {
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

	item, err := h.cartService.AddItem(
		c.Request.Context(),
		pgtype.UUID{
			Bytes: cartID,
			Valid: true,
		},
		pgtype.UUID{
			Bytes: variantID,
			Valid: true,
		},
		request.Quantity,
	)
	if err != nil {
		switch {
		case errors.Is(err, cart.ErrCartNotFound):
			writeError(
				c,
				http.StatusNotFound,
				"CART_NOT_FOUND",
				"cart not found",
			)
		case errors.Is(err, cart.ErrCartNotActive):
			writeError(
				c,
				http.StatusConflict,
				"CART_NOT_ACTIVE",
				"cart is not active",
			)
		case errors.Is(err, cart.ErrInvalidQuantity):
			writeError(
				c,
				http.StatusBadRequest,
				"INVALID_QUANTITY",
				"quantity must be greater than zero",
			)
		default:
			writeError(
				c,
				http.StatusInternalServerError,
				"INTERNAL_ERROR",
				"internal server error",
			)
		}
		return
	}

	c.JSON(
		http.StatusCreated,
		gin.H{
			"id":         item.ID,
			"cart_id":    item.CartID,
			"variant_id": item.VariantID,
			"quantity":   item.Quantity,
			"created_at": item.CreatedAt,
			"updated_at": item.UpdatedAt,
		},
	)
}

func (h *Handler) GetCart(c *gin.Context) {
	cartID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(
			c,
			http.StatusBadRequest,
			"INVALID_CART_ID",
			"invalid cart id",
		)
		return
	}

	id := pgtype.UUID{
		Bytes: cartID,
		Valid: true,
	}

	cartRecord, err := h.cartService.GetCart(
		c.Request.Context(),
		id,
	)
	if err != nil {
		if errors.Is(err, cart.ErrCartNotFound) {
			writeError(
				c,
				http.StatusNotFound,
				"CART_NOT_FOUND",
				"cart not found",
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

	items, err := h.cartService.GetCartItems(
		c.Request.Context(),
		id,
	)
	if err != nil {
		writeError(
			c,
			http.StatusInternalServerError,
			"INTERNAL_ERROR",
			"internal server error",
		)
		return
	}

	c.JSON(
		http.StatusOK,
		gin.H{
			"id":         cartRecord.ID,
			"status":     cartRecord.Status,
			"created_at": cartRecord.CreatedAt,
			"updated_at": cartRecord.UpdatedAt,
			"items":      items,
		},
	)
}

func (h *Handler) UpdateCartItem(c *gin.Context) {
	cartID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(
			c,
			http.StatusBadRequest,
			"INVALID_CART_ID",
			"invalid cart id",
		)
		return
	}

	variantID, err := uuid.Parse(c.Param("variant_id"))
	if err != nil {
		writeError(
			c,
			http.StatusBadRequest,
			"INVALID_VARIANT_ID",
			"invalid variant_id",
		)
		return
	}

	var request UpdateCartItemRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(
			c,
			http.StatusBadRequest,
			"INVALID_REQUEST",
			"invalid request",
		)
		return
	}

	item, err := h.cartService.UpdateItemQuantity(
		c.Request.Context(),
		pgtype.UUID{
			Bytes: cartID,
			Valid: true,
		},
		pgtype.UUID{
			Bytes: variantID,
			Valid: true,
		},
		request.Quantity,
	)
	if err != nil {
		switch {
		case errors.Is(err, cart.ErrCartNotFound):
			writeError(
				c,
				http.StatusNotFound,
				"CART_NOT_FOUND",
				"cart not found",
			)
		case errors.Is(err, cart.ErrCartNotActive):
			writeError(
				c,
				http.StatusConflict,
				"CART_NOT_ACTIVE",
				"cart is not active",
			)
		case errors.Is(err, cart.ErrCartItemNotFound):
			writeError(
				c,
				http.StatusNotFound,
				"CART_ITEM_NOT_FOUND",
				"cart item not found",
			)
		case errors.Is(err, cart.ErrInvalidQuantity):
			writeError(
				c,
				http.StatusBadRequest,
				"INVALID_QUANTITY",
				"quantity must be greater than zero",
			)
		default:
			writeError(
				c,
				http.StatusInternalServerError,
				"INTERNAL_ERROR",
				"internal server error",
			)
		}
		return
	}

	c.JSON(
		http.StatusOK,
		gin.H{
			"id":         item.ID,
			"cart_id":    item.CartID,
			"variant_id": item.VariantID,
			"quantity":   item.Quantity,
			"created_at": item.CreatedAt,
			"updated_at": item.UpdatedAt,
		},
	)
}

func (h *Handler) RemoveCartItem(c *gin.Context) {
	cartID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(
			c,
			http.StatusBadRequest,
			"INVALID_CART_ID",
			"invalid cart id",
		)
		return
	}

	variantID, err := uuid.Parse(c.Param("variant_id"))
	if err != nil {
		writeError(
			c,
			http.StatusBadRequest,
			"INVALID_VARIANT_ID",
			"invalid variant_id",
		)
		return
	}

	err = h.cartService.RemoveItem(
		c.Request.Context(),
		pgtype.UUID{
			Bytes: cartID,
			Valid: true,
		},
		pgtype.UUID{
			Bytes: variantID,
			Valid: true,
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, cart.ErrCartNotFound):
			writeError(
				c,
				http.StatusNotFound,
				"CART_NOT_FOUND",
				"cart not found",
			)
		case errors.Is(err, cart.ErrCartNotActive):
			writeError(
				c,
				http.StatusConflict,
				"CART_NOT_ACTIVE",
				"cart is not active",
			)
		default:
			writeError(
				c,
				http.StatusInternalServerError,
				"INTERNAL_ERROR",
				"internal server error",
			)
		}
		return
	}

	c.Status(http.StatusNoContent)
}
