package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/TomatoesSuck/distributed-order-processing/payment-service/internal/model"
	"github.com/TomatoesSuck/distributed-order-processing/payment-service/internal/service"
)

type PaymentHandler struct {
	svc *service.PaymentService
}

func NewPaymentHandler(svc *service.PaymentService) *PaymentHandler {
	return &PaymentHandler{svc: svc}
}

func (h *PaymentHandler) Register(r *gin.Engine) {
	r.GET("/payments/:order_id", h.Get)
	r.POST("/payments", h.Create)
}

type createPaymentRequest struct {
	OrderID uint64  `json:"order_id" binding:"required"`
	Amount  float64 `json:"amount"   binding:"required,gt=0"`
}

func (h *PaymentHandler) Get(c *gin.Context) {
	orderID, err := strconv.ParseUint(c.Param("order_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order_id", "code": "INVALID_ORDER_ID"})
		return
	}

	p, err := h.svc.GetByOrderID(c.Request.Context(), orderID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "payment not found", "code": "NOT_FOUND"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "code": "GET_FAILED"})
		return
	}

	c.JSON(http.StatusOK, p)
}

func (h *PaymentHandler) Create(c *gin.Context) {
	var req createPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": "INVALID_REQUEST"})
		return
	}

	p := &model.Payment{
		OrderID: req.OrderID,
		Amount:  req.Amount,
	}

	if err := h.svc.CreatePayment(c.Request.Context(), p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "code": "CREATE_FAILED"})
		return
	}

	c.JSON(http.StatusCreated, p)
}
