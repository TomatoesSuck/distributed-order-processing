package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/TomatoesSuck/distributed-order-processing/inventory-service/internal/model"
	"github.com/TomatoesSuck/distributed-order-processing/inventory-service/internal/service"
)

type InventoryHandler struct {
	svc *service.InventoryService
}

func NewInventoryHandler(svc *service.InventoryService) *InventoryHandler {
	return &InventoryHandler{svc: svc}
}

func (h *InventoryHandler) Register(r *gin.Engine) {
	r.GET("/inventory/:product_id", h.Get)
	r.POST("/inventory", h.Create)
	r.PUT("/inventory/:product_id", h.Update)
}

type createInventoryRequest struct {
	ProductID    uint64 `json:"product_id"    binding:"required"`
	AvailableQty int    `json:"available_qty" binding:"min=0"`
}

type updateInventoryRequest struct {
	AvailableQty *int `json:"available_qty" binding:"required,min=0"`
}

func (h *InventoryHandler) Get(c *gin.Context) {
	productID, err := strconv.ParseUint(c.Param("product_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product_id", "code": "INVALID_PRODUCT_ID"})
		return
	}

	inv, err := h.svc.GetByProductID(c.Request.Context(), productID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "product not found", "code": "NOT_FOUND"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "code": "GET_FAILED"})
		return
	}

	c.JSON(http.StatusOK, inv)
}

func (h *InventoryHandler) Create(c *gin.Context) {
	var req createInventoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": "INVALID_REQUEST"})
		return
	}

	inv := &model.Inventory{
		ProductID:    req.ProductID,
		AvailableQty: req.AvailableQty,
	}

	if err := h.svc.CreateSKU(c.Request.Context(), inv); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "code": "CREATE_FAILED"})
		return
	}

	c.JSON(http.StatusCreated, inv)
}

func (h *InventoryHandler) Update(c *gin.Context) {
	productID, err := strconv.ParseUint(c.Param("product_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product_id", "code": "INVALID_PRODUCT_ID"})
		return
	}

	var req updateInventoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": "INVALID_REQUEST"})
		return
	}

	if err := h.svc.UpdateAvailableQty(c.Request.Context(), productID, *req.AvailableQty); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "product not found", "code": "NOT_FOUND"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "code": "UPDATE_FAILED"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"product_id": productID, "available_qty": *req.AvailableQty})
}
