package categories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"personal-finance-manager/internal/models"
)

var ErrNotFound = errors.New("category not found")

type Repository interface {
	List(ctx context.Context, userID int64) ([]models.Category, error)
	GetByID(ctx context.Context, id, userID int64) (*models.Category, error)
	Create(ctx context.Context, c *models.Category) error
	Delete(ctx context.Context, id, userID int64) error
}

type repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) Repository { return &repository{db: db} }

func (r *repository) List(ctx context.Context, userID int64) ([]models.Category, error) {
	const q = `SELECT id, user_id, name, type, color, created_at FROM categories WHERE user_id=$1 ORDER BY name`
	rows, err := r.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	defer rows.Close()

	var cats []models.Category
	for rows.Next() {
		var c models.Category
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &c.Type, &c.Color, &c.CreatedAt); err != nil {
			return nil, err
		}
		cats = append(cats, c)
	}
	return cats, rows.Err()
}

func (r *repository) GetByID(ctx context.Context, id, userID int64) (*models.Category, error) {
	const q = `SELECT id, user_id, name, type, color, created_at FROM categories WHERE id=$1 AND user_id=$2`
	c := &models.Category{}
	err := r.db.QueryRowContext(ctx, q, id, userID).Scan(&c.ID, &c.UserID, &c.Name, &c.Type, &c.Color, &c.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return c, err
}

func (r *repository) Create(ctx context.Context, c *models.Category) error {
	const q = `
		INSERT INTO categories (user_id, name, type, color)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at`
	return r.db.QueryRowContext(ctx, q, c.UserID, c.Name, c.Type, c.Color).Scan(&c.ID, &c.CreatedAt)
}

func (r *repository) Delete(ctx context.Context, id, userID int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM categories WHERE id=$1 AND user_id=$2`, id, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
