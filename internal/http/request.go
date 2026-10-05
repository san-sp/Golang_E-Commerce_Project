package http

type PaymentWebhookRequest struct {
	EventID   string `json:"event_id" binding:"required"`
	EventType string `json:"event_type" binding:"required"`
	PaymentID string `json:"payment_id" binding:"required,uuid"`
}

type CreatePaymentRequest struct {
	OrderID  string `json:"order_id" binding:"required,uuid"`
	Provider string `json:"provider" binding:"required"`
	Currency string `json:"currency" binding:"required"`
}

type CreateReservationRequest struct {
	VariantID string `json:"variant_id" binding:"required,uuid"`
	Quantity  int64  `json:"quantity" binding:"required,gt=0"`
}

type AddCartItemRequest struct {
	VariantID string `json:"variant_id" binding:"required,uuid"`
	Quantity  int64  `json:"quantity" binding:"required,gt=0"`
}

type UpdateCartItemRequest struct {
	Quantity int64 `json:"quantity" binding:"required,gt=0"`
}

type CheckoutRequest struct {
	CartID   string `json:"cart_id" binding:"required,uuid"`
	Currency string `json:"currency" binding:"required"`
	Provider string `json:"provider" binding:"required"`
}
