package model

import "time"

type BaseModel struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

const (
	OrderStatusPending           = "PENDING"
	OrderStatusInventoryReserved = "INVENTORY_RESERVED"
	OrderStatusPaid              = "PAID"
	OrderStatusConfirmed         = "CONFIRMED"
	OrderStatusFailed            = "FAILED"
	OrderStatusCompensated       = "COMPENSATED"
)

type Order struct {
	BaseModel
	UserID      uint64  `gorm:"not null;index"                json:"user_id"`
	ProductID   uint64  `gorm:"not null"                      json:"product_id"`
	Quantity    int     `gorm:"not null"                      json:"quantity"`
	TotalAmount float64 `gorm:"type:decimal(12,2);not null"   json:"total_amount"`
	Status      string  `gorm:"type:varchar(32);not null;default:PENDING;index" json:"status"`
}
