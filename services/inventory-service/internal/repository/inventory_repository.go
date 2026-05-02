package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/TomatoesSuck/distributed-order-processing/inventory-service/internal/model"
)

type InventoryRepository struct {
	db *gorm.DB
}

func NewInventoryRepository(db *gorm.DB) *InventoryRepository {
	return &InventoryRepository{db: db}
}

func (r *InventoryRepository) Create(ctx context.Context, inv *model.Inventory) error {
	if err := r.db.WithContext(ctx).Create(inv).Error; err != nil {
		return fmt.Errorf("create inventory: %w", err)
	}
	return nil
}

func (r *InventoryRepository) GetByProductID(ctx context.Context, productID uint64) (*model.Inventory, error) {
	var inv model.Inventory
	if err := r.db.WithContext(ctx).Where("product_id = ?", productID).First(&inv).Error; err != nil {
		return nil, fmt.Errorf("get inventory product %d: %w", productID, err)
	}
	return &inv, nil
}

func (r *InventoryRepository) UpdateAvailableQty(ctx context.Context, productID uint64, qty int) error {
	result := r.db.WithContext(ctx).
		Model(&model.Inventory{}).
		Where("product_id = ?", productID).
		Update("available_qty", qty)
	if result.Error != nil {
		return fmt.Errorf("update available_qty product %d: %w", productID, result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("inventory for product %d not found", productID)
	}
	return nil
}

// SeedIfNotExists creates the record only when product_id is absent (idempotent).
func (r *InventoryRepository) SeedIfNotExists(ctx context.Context, productID uint64, availableQty int) error {
	inv := model.Inventory{
		ProductID:    productID,
		AvailableQty: availableQty,
	}
	result := r.db.WithContext(ctx).Where("product_id = ?", productID).FirstOrCreate(&inv)
	if result.Error != nil {
		return fmt.Errorf("seed product %d: %w", productID, result.Error)
	}
	return nil
}
