package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/TomatoesSuck/distributed-order-processing/order-service/internal/model"
	"github.com/TomatoesSuck/distributed-order-processing/order-service/internal/repository"
)

type AdminHandler struct {
	sagaRepo  *repository.SagaRepository
	orderRepo *repository.OrderRepository
}

func NewAdminHandler(sagaRepo *repository.SagaRepository, orderRepo *repository.OrderRepository) *AdminHandler {
	return &AdminHandler{sagaRepo: sagaRepo, orderRepo: orderRepo}
}

func (h *AdminHandler) Register(r *gin.Engine) {
	r.GET("/admin/sagas", h.ListSagas)
	r.GET("/admin/sagas/:saga_id", h.GetSaga)
}

func (h *AdminHandler) ListSagas(c *gin.Context) {
	status := c.Query("status")

	var (
		states []model.SagaState
		err    error
	)
	if status == "" {
		states, err = h.sagaRepo.List(c.Request.Context())
	} else {
		states, err = h.sagaRepo.ListByStatus(c.Request.Context(), status)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "code": "LIST_FAILED"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": len(states), "sagas": states})
}

func (h *AdminHandler) GetSaga(c *gin.Context) {
	sagaID := c.Param("saga_id")
	if sagaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "saga_id is required", "code": "INVALID_SAGA_ID"})
		return
	}

	state, err := h.sagaRepo.GetBySagaID(c.Request.Context(), sagaID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "saga not found", "code": "NOT_FOUND"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "code": "GET_FAILED"})
		return
	}

	order, err := h.orderRepo.GetByID(c.Request.Context(), state.OrderID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "code": "GET_ORDER_FAILED"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"saga": state, "order": order})
}
