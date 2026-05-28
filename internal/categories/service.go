package categories

import (
	"context"

	"personal-finance-manager/internal/models"
)

type Service interface {
	List(ctx context.Context, userID int64) ([]models.Category, error)
	Create(ctx context.Context, userID int64, req *models.CreateCategoryRequest) (*models.Category, error)
	Delete(ctx context.Context, id, userID int64) error
}

type service struct{ repo Repository }

func NewService(repo Repository) Service { return &service{repo: repo} }

func (s *service) List(ctx context.Context, userID int64) ([]models.Category, error) {
	return s.repo.List(ctx, userID)
}

func (s *service) Create(ctx context.Context, userID int64, req *models.CreateCategoryRequest) (*models.Category, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if req.Color == "" {
		req.Color = "#6B7280"
	}
	c := &models.Category{
		UserID: userID,
		Name:   req.Name,
		Type:   req.Type,
		Color:  req.Color,
	}
	if err := s.repo.Create(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *service) Delete(ctx context.Context, id, userID int64) error {
	return s.repo.Delete(ctx, id, userID)
}
