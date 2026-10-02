package http

type PaymentWebhookRequest struct {
	EventID   string `json:"event_id" binding:"required"`
	EventType string `json:"event_type" binding:"required"`
	PaymentID string `json:"payment_id" binding:"required,uuid"`
}

type CreatePaymentRequest struct {
	ReservationID string `json:"reservation_id" binding:"required"`
	Provider      string `json:"provider" binding:"required"`
	Amount        int64  `json:"amount" binding:"required,gt=0"`
	Currency      string `json:"currency" binding:"required"`
}
