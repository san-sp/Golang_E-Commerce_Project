package http

type PaymentWebhookRequest struct {
	EventID   string `json:"event_id" binding:"required"`
	EventType string `json:"event_type" binding:"required"`
	PaymentID string `json:"payment_id" binding:"required,uuid"`
}
