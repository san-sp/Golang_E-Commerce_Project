package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/san-sp/Golang_E-Commerce_Project/internal/cart"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/checkout"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/database/db"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/order"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/payment"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/product"
	productpkg "github.com/san-sp/Golang_E-Commerce_Project/internal/product"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/reservation"
)

type Handler struct {
	paymentService     *payment.Service
	refundService      *payment.RefundService
	reservationService *reservation.Service
	cartService        *cart.Service
	checkoutService    *checkout.Service
	orderService       *order.Service
	productService     *product.Service
}

func NewHandler(
	paymentService *payment.Service,
	refundService *payment.RefundService,
	reservationService *reservation.Service,
	cartService *cart.Service,
	checkoutService *checkout.Service,
	orderService *order.Service,
	productService *product.Service,
) *Handler {
	return &Handler{
		paymentService:     paymentService,
		refundService:      refundService,
		reservationService: reservationService,
		cartService:        cartService,
		checkoutService:    checkoutService,
		orderService:       orderService,
		productService:     productService,
	}
}

// Product
func (h *Handler) GetProduct(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid product ID",
		})
		return
	}

	product, err := h.productService.GetProduct(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, productpkg.ErrProductNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "product not found",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "internal server error",
		})
		return
	}

	c.JSON(http.StatusOK, product)
}

func (h *Handler) ListProducts(c *gin.Context) {
	page := 1
	limit := 20
	sort := c.Query("sort")
	search := c.Query("search")

	if value := c.Query("page"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid page",
			})
			return
		}

		page = parsed
	}

	if value := c.Query("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid limit",
			})
			return
		}

		limit = parsed
	}

	var (
		products []db.Product
		err      error
	)

	if search != "" {
		products, err = h.productService.SearchProducts(
			c.Request.Context(),
			search,
			page,
			limit,
		)
	} else {
		products, err = h.productService.ListProducts(
			c.Request.Context(),
			page,
			limit,
			sort,
		)
	}
	if err != nil {
		switch {
		case errors.Is(err, productpkg.ErrInvalidPage):
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})

		case errors.Is(err, productpkg.ErrInvalidLimit):
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})

		case errors.Is(err, productpkg.ErrInvalidProductSort):
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})

		case errors.Is(err, productpkg.ErrInvalidProductSearch):
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "internal server error",
			})
		}

		return
	}

	c.JSON(http.StatusOK, products)
}

func (h *Handler) ListProductVariants(c *gin.Context) {
	productID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid product ID",
		})
		return
	}

	variants, err := h.productService.ListProductVariants(
		c.Request.Context(),
		productID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "internal server error",
		})
		return
	}

	c.JSON(http.StatusOK, variants)
}

// Cart
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

// Checkout
func (h *Handler) Checkout(c *gin.Context) {
	var request CheckoutRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(
			c,
			http.StatusBadRequest,
			"INVALID_REQUEST",
			"invalid request",
		)
		return
	}

	cartID, err := uuid.Parse(request.CartID)
	if err != nil {
		writeError(
			c,
			http.StatusBadRequest,
			"INVALID_CART_ID",
			"invalid cart_id",
		)
		return
	}

	result, err := h.checkoutService.Checkout(
		c.Request.Context(),
		pgtype.UUID{
			Bytes: cartID,
			Valid: true,
		},
		request.Currency,
		request.Provider,
	)
	if err != nil {
		switch {
		case errors.Is(err, checkout.ErrCartNotFound):
			writeError(
				c,
				http.StatusNotFound,
				"CART_NOT_FOUND",
				"cart not found",
			)

		case errors.Is(err, checkout.ErrCartNotActive):
			writeError(
				c,
				http.StatusConflict,
				"CART_NOT_ACTIVE",
				"cart is not active",
			)

		case errors.Is(err, checkout.ErrCartEmpty):
			writeError(
				c,
				http.StatusConflict,
				"CART_EMPTY",
				"cart is empty",
			)

		case errors.Is(err, payment.ErrProviderFailed):
			writeError(
				c,
				http.StatusBadGateway,
				"PAYMENT_FAILED",
				"payment provider failed",
			)

		case errors.Is(err, payment.ErrProviderUnknown):
			writeError(
				c,
				http.StatusAccepted,
				"PAYMENT_OUTCOME_UNKNOWN",
				"payment outcome is unknown; await payment confirmation",
			)

		case errors.Is(err, reservation.ErrInsufficientStock):
			writeError(
				c,
				http.StatusConflict,
				"INSUFFICIENT_STOCK",
				"insufficient stock",
			)

		default:
			writeError(
				c,
				http.StatusInternalServerError,
				"CHECKOUT_FAILED",
				"checkout failed",
			)
		}

		return
	}

	c.JSON(
		http.StatusCreated,
		gin.H{
			"order": gin.H{
				"id":           result.Order.ID,
				"status":       result.Order.Status,
				"total_amount": result.Order.TotalAmount,
				"currency":     result.Order.Currency,
				"created_at":   result.Order.CreatedAt,
				"updated_at":   result.Order.UpdatedAt,
			},
			"order_items":  result.OrderItems,
			"reservations": result.Reservations,
			"payment": gin.H{
				"id":                  result.Payment.ID,
				"status":              result.Payment.Status,
				"provider":            result.Payment.Provider,
				"provider_payment_id": result.Payment.ProviderPaymentID,
				"amount":              result.Payment.Amount,
				"currency":            result.Payment.Currency,
			},
		},
	)
}

// Reservation
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

// Payment
func (h *Handler) CreatePayment(c *gin.Context) {
	var request CreatePaymentRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(
			c,
			http.StatusBadRequest,
			"INVALID_REQUEST",
			"invalid request",
		)
		return
	}

	orderID, err := uuid.Parse(request.OrderID)
	if err != nil {
		writeError(
			c,
			http.StatusBadRequest,
			"INVALID_ORDER_ID",
			"invalid order_id",
		)
		return
	}

	orderRecord, _, err := h.orderService.GetOrder(
		c.Request.Context(),
		orderID,
	)
	if err != nil {
		if errors.Is(err, order.ErrOrderNotFound) {
			writeError(
				c,
				http.StatusNotFound,
				"ORDER_NOT_FOUND",
				"order not found",
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

	if orderRecord.Status != "PENDING" {
		writeError(
			c,
			http.StatusConflict,
			"ORDER_NOT_PENDING",
			"order is not pending",
		)
		return
	}

	if request.Currency != orderRecord.Currency {
		writeError(
			c,
			http.StatusBadRequest,
			"CURRENCY_MISMATCH",
			"currency does not match order currency",
		)
		return
	}

	paymentRecord, err := h.paymentService.CreatePayment(
		c.Request.Context(),
		pgtype.UUID{
			Bytes: orderID,
			Valid: true,
		},
		request.Provider,
		orderRecord.TotalAmount,
		orderRecord.Currency,
	)
	if err != nil {
		if errors.Is(err, payment.ErrProviderFailed) {
			writeError(
				c,
				http.StatusBadGateway,
				"PAYMENT_FAILED",
				"payment provider failed",
			)
			return
		}

		if errors.Is(err, payment.ErrProviderUnknown) {
			writeError(
				c,
				http.StatusAccepted,
				"PAYMENT_OUTCOME_UNKNOWN",
				"payment outcome is unknown; await payment confirmation",
			)
			return
		}

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
			"id":                  paymentRecord.ID,
			"status":              paymentRecord.Status,
			"order_id":            paymentRecord.OrderID,
			"provider":            paymentRecord.Provider,
			"provider_payment_id": paymentRecord.ProviderPaymentID,
			"amount":              paymentRecord.Amount,
			"currency":            paymentRecord.Currency,
		},
	)
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

// Refund
func (h *Handler) CreateRefund(c *gin.Context) {
	paymentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(
			c,
			http.StatusBadRequest,
			"INVALID_PAYMENT_ID",
			"invalid payment id",
		)
		return
	}

	idempotencyKey := c.GetHeader("Idempotency-Key")
	if idempotencyKey == "" {
		writeError(
			c,
			http.StatusBadRequest,
			"INVALID_IDEMPOTENCY_KEY",
			"Idempotency-Key header is required",
		)
		return
	}

	refundRecord, err := h.refundService.CreateRefund(
		c.Request.Context(),
		pgtype.UUID{
			Bytes: paymentID,
			Valid: true,
		},
		idempotencyKey,
	)
	if err != nil {
		if errors.Is(err, payment.ErrInvalidIdempotency) {
			writeError(
				c,
				http.StatusBadRequest,
				"INVALID_IDEMPOTENCY_KEY",
				"invalid idempotency key",
			)
			return
		}

		if errors.Is(err, payment.ErrPaymentNotFound) {
			writeError(
				c,
				http.StatusNotFound,
				"PAYMENT_NOT_FOUND",
				"payment not found",
			)
			return
		}

		if errors.Is(err, payment.ErrPaymentNotRefundable) {
			writeError(
				c,
				http.StatusConflict,
				"PAYMENT_NOT_REFUNDABLE",
				"payment is not refundable",
			)
			return
		}

		if errors.Is(err, payment.ErrOrderNotRefundable) {
			writeError(
				c,
				http.StatusConflict,
				"ORDER_NOT_REFUNDABLE",
				"order is not refundable",
			)
			return
		}

		if errors.Is(err, payment.ErrRefundAlreadyExists) {
			writeError(
				c,
				http.StatusConflict,
				"REFUND_ALREADY_EXISTS",
				"refund already exists",
			)
			return
		}

		if errors.Is(err, payment.ErrProviderFailed) {
			writeError(
				c,
				http.StatusBadGateway,
				"REFUND_FAILED",
				"payment provider failed to process refund",
			)
			return
		}

		if errors.Is(err, payment.ErrProviderUnknown) {
			c.JSON(
				http.StatusAccepted,
				gin.H{
					"id":                 refundRecord.ID,
					"payment_id":         refundRecord.PaymentID,
					"status":             refundRecord.Status,
					"provider_refund_id": refundRecord.ProviderRefundID,
					"amount":             refundRecord.Amount,
					"currency":           refundRecord.Currency,
					"idempotency_key":    refundRecord.IdempotencyKey,
				},
			)
			return
		}

		writeError(
			c,
			http.StatusInternalServerError,
			"REFUND_FAILED",
			"refund processing failed",
		)
		return
	}

	c.JSON(
		http.StatusOK,
		gin.H{
			"id":                 refundRecord.ID,
			"payment_id":         refundRecord.PaymentID,
			"status":             refundRecord.Status,
			"provider_refund_id": refundRecord.ProviderRefundID,
			"amount":             refundRecord.Amount,
			"currency":           refundRecord.Currency,
			"idempotency_key":    refundRecord.IdempotencyKey,
		},
	)
}

// Order
func (h *Handler) GetOrder(c *gin.Context) {
	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(
			c,
			http.StatusBadRequest,
			"INVALID_ORDER_ID",
			"invalid order_id",
		)
		return
	}

	foundOrder, items, err := h.orderService.GetOrder(
		c.Request.Context(),
		orderID,
	)
	if err != nil {
		if errors.Is(err, order.ErrOrderNotFound) {
			writeError(
				c,
				http.StatusNotFound,
				"ORDER_NOT_FOUND",
				"order not found",
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

	responseItems := make([]gin.H, 0, len(items))

	for _, item := range items {
		responseItems = append(
			responseItems,
			gin.H{
				"id":         item.ID,
				"variant_id": item.VariantID,
				"quantity":   item.Quantity,
				"unit_price": item.UnitPrice,
				"created_at": item.CreatedAt,
				"updated_at": item.UpdatedAt,
			},
		)
	}

	c.JSON(
		http.StatusOK,
		gin.H{
			"id":           foundOrder.ID,
			"status":       foundOrder.Status,
			"total_amount": foundOrder.TotalAmount,
			"currency":     foundOrder.Currency,
			"items":        responseItems,
			"created_at":   foundOrder.CreatedAt,
			"updated_at":   foundOrder.UpdatedAt,
		},
	)
}

func (h *Handler) CancelOrder(c *gin.Context) {
	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(
			c,
			http.StatusBadRequest,
			"INVALID_ORDER_ID",
			"invalid order id",
		)
		return
	}

	cancelledOrder, reservations, err := h.checkoutService.CancelOrder(
		c.Request.Context(),
		pgtype.UUID{
			Bytes: orderID,
			Valid: true,
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, order.ErrOrderNotFound):
			writeError(
				c,
				http.StatusNotFound,
				"ORDER_NOT_FOUND",
				"order not found",
			)

		case errors.Is(err, order.ErrInvalidOrderState):
			writeError(
				c,
				http.StatusConflict,
				"ORDER_CANNOT_BE_CANCELLED",
				"order cannot be cancelled",
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
			"order":        cancelledOrder,
			"reservations": reservations,
		},
	)
}
