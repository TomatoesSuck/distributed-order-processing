package service

import (
	"context"
	"fmt"

	shared "github.com/TomatoesSuck/distributed-order-processing/shared"

	"github.com/TomatoesSuck/distributed-order-processing/payment-service/internal/model"
	"github.com/TomatoesSuck/distributed-order-processing/payment-service/internal/repository"
)

type PaymentService struct {
	repo repository.PaymentRepoIface
}

func NewPaymentService(repo repository.PaymentRepoIface) *PaymentService {
	return &PaymentService{repo: repo}
}

func (s *PaymentService) CreatePayment(ctx context.Context, p *model.Payment) error {
	p.Status = model.PaymentStatusPending
	p.TransactionID = shared.NewUUID()
	if err := s.repo.Create(ctx, p); err != nil {
		return fmt.Errorf("create payment: %w", err)
	}
	return nil
}

func (s *PaymentService) GetByOrderID(ctx context.Context, orderID uint64) (*model.Payment, error) {
	return s.repo.GetByOrderID(ctx, orderID)
}
