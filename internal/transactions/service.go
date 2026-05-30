package transactions

import (
	"context"
	"errors"
	"fmt"
	"time"

	"personal-finance-manager/internal/models"
)

type Service interface {
	List(ctx context.Context, userID int64, f Filter) ([]models.Transaction, int, error)
	GetByID(ctx context.Context, id, userID int64) (*models.Transaction, error)
	Create(ctx context.Context, userID int64, req *models.CreateTransactionRequest) (*models.Transaction, error)
	Update(ctx context.Context, id, userID int64, req *models.UpdateTransactionRequest) (*models.Transaction, error)
	Delete(ctx context.Context, id, userID int64) error
	Balance(ctx context.Context, userID int64, currency string) (*models.BalanceResponse, error)
}

type OnBalanceChange func(ctx context.Context, userID int64, currency string)

type service struct {
	repo            Repository
	onBalanceChange OnBalanceChange
}

func NewService(repo Repository, onBalanceChange OnBalanceChange) Service {
	return &service{repo: repo, onBalanceChange: onBalanceChange}
}

func (s *service) List(ctx context.Context, userID int64, f Filter) ([]models.Transaction, int, error) {
	return s.repo.List(ctx, userID, f)
}

func (s *service) GetByID(ctx context.Context, id, userID int64) (*models.Transaction, error) {
	return s.repo.GetByID(ctx, id, userID)
}

func (s *service) Create(ctx context.Context, userID int64, req *models.CreateTransactionRequest) (*models.Transaction, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	date, err := parseDate(req.Date)
	if err != nil {
		return nil, models.ErrValidation("invalid date format, expected YYYY-MM-DD")
	}

	t := &models.Transaction{
		UserID:      userID,
		CategoryID:  req.CategoryID,
		Amount:      req.Amount,
		Currency:    req.Currency,
		Type:        req.Type,
		Description: req.Description,
		Date:        date,
	}
	if err := s.repo.Create(ctx, t); err != nil {
		return nil, fmt.Errorf("create transaction: %w", err)
	}

	if s.onBalanceChange != nil {
		go s.onBalanceChange(context.Background(), userID, t.Currency)
	}
	return t, nil
}

func (s *service) Update(ctx context.Context, id, userID int64, req *models.UpdateTransactionRequest) (*models.Transaction, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	existing, err := s.repo.GetByID(ctx, id, userID)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	date, err := parseDate(req.Date)
	if err != nil {
		return nil, models.ErrValidation("invalid date format, expected YYYY-MM-DD")
	}

	existing.CategoryID = req.CategoryID
	existing.Amount = req.Amount
	existing.Currency = req.Currency
	existing.Type = req.Type
	existing.Description = req.Description
	existing.Date = date

	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, err
	}

	if s.onBalanceChange != nil {
		go s.onBalanceChange(context.Background(), userID, existing.Currency)
	}
	return existing, nil
}

func (s *service) Delete(ctx context.Context, id, userID int64) error {
	tx, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id, userID); err != nil {
		return err
	}
	if s.onBalanceChange != nil {
		go s.onBalanceChange(context.Background(), userID, tx.Currency)
	}
	return nil
}

func (s *service) Balance(ctx context.Context, userID int64, currency string) (*models.BalanceResponse, error) {
	if currency == "" {
		currency = "PLN"
	}
	return s.repo.Balance(ctx, userID, currency)
}

func parseDate(s string) (time.Time, error) {
	if s == "" {
		return time.Now().Truncate(24 * time.Hour), nil
	}
	return time.Parse("2006-01-02", s)
}
