package model

import "time"

type BaseModel struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Inventory struct {
	BaseModel
	ProductID    uint64 `gorm:"not null;uniqueIndex"        json:"product_id"`
	AvailableQty int    `gorm:"not null"                    json:"available_qty"`
	ReservedQty  int    `gorm:"not null;default:0"          json:"reserved_qty"`
	Version      int    `gorm:"not null;default:0"          json:"version"`
}
