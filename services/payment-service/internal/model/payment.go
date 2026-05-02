package model

import "time"

type BaseModel struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

const (
	PaymentStatusPending  = "PENDING"
	PaymentStatusSuccess  = "SUCCESS"
	PaymentStatusFailed   = "FAILED"
	PaymentStatusRefunded = "REFUNDED"
)

type Payment struct {
	BaseModel
	OrderID       uint64  `gorm:"not null;uniqueIndex"          json:"order_id"`
	Amount        float64 `gorm:"type:decimal(12,2);not null"   json:"amount"`
	Status        string  `gorm:"type:varchar(32);not null"     json:"status"`
	TransactionID string  `gorm:"type:varchar(64)"              json:"transaction_id"`
}
