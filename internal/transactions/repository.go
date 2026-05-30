package transactions

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"personal-finance-manager/internal/models"
)

var ErrNotFound = errors.New("transaction not found")

type Filter struct {
	Type     string
	Category int64
	From     time.Time
	To       time.Time
	Page     int
	PerPage  int
}

type Repository interface {
	List(ctx context.Context, userID int64, f Filter) ([]models.Transaction, int, error)
	GetByID(ctx context.Context, id, userID int64) (*models.Transaction, error)
	Create(ctx context.Context, t *models.Transaction) error
	Update(ctx context.Context, t *models.Transaction) error
	Delete(ctx context.Context, id, userID int64) error
	Balance(ctx context.Context, userID int64, currency string) (*models.BalanceResponse, error)

	AddAttachment(ctx context.Context, a *models.Attachment) error
	ListAttachments(ctx context.Context, txID, userID int64) ([]models.Attachment, error)
	GetAttachmentByID(ctx context.Context, id, userID int64) (*models.Attachment, error)
}

type repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) Repository { return &repository{db: db} }

func (r *repository) List(ctx context.Context, userID int64, f Filter) ([]models.Transaction, int, error) {
	args := []interface{}{userID}
	where := "WHERE t.user_id = $1"
	idx := 2

	if f.Type != "" {
		where += fmt.Sprintf(" AND t.type = $%d", idx)
		args = append(args, f.Type)
		idx++
	}
	if f.Category != 0 {
		where += fmt.Sprintf(" AND t.category_id = $%d", idx)
		args = append(args, f.Category)
		idx++
	}
	if !f.From.IsZero() {
		where += fmt.Sprintf(" AND t.date >= $%d", idx)
		args = append(args, f.From)
		idx++
	}
	if !f.To.IsZero() {
		where += fmt.Sprintf(" AND t.date <= $%d", idx)
		args = append(args, f.To)
		idx++
	}

	countQ := "SELECT COUNT(*) FROM transactions t " + where
	var total int
	if err := r.db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count transactions: %w", err)
	}

	if f.PerPage == 0 {
		f.PerPage = 20
	}
	if f.Page < 1 {
		f.Page = 1
	}
	offset := (f.Page - 1) * f.PerPage

	q := "SELECT t.id, t.user_id, t.category_id, t.amount, t.currency, t.type, t.description, t.date, t.created_at, t.updated_at " +
		"FROM transactions t " + where +
		fmt.Sprintf(" ORDER BY t.date DESC, t.id DESC LIMIT $%d OFFSET $%d", idx, idx+1)
	args = append(args, f.PerPage, offset)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list transactions: %w", err)
	}
	defer rows.Close()

	var txs []models.Transaction
	for rows.Next() {
		var t models.Transaction
		if err := rows.Scan(
			&t.ID, &t.UserID, &t.CategoryID, &t.Amount, &t.Currency,
			&t.Type, &t.Description, &t.Date, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan transaction: %w", err)
		}
		txs = append(txs, t)
	}
	return txs, total, rows.Err()
}

func (r *repository) GetByID(ctx context.Context, id, userID int64) (*models.Transaction, error) {
	const q = `
		SELECT id, user_id, category_id, amount, currency, type, description, date, created_at, updated_at
		FROM transactions WHERE id = $1 AND user_id = $2`

	t := &models.Transaction{}
	err := r.db.QueryRowContext(ctx, q, id, userID).Scan(
		&t.ID, &t.UserID, &t.CategoryID, &t.Amount, &t.Currency,
		&t.Type, &t.Description, &t.Date, &t.CreatedAt, &t.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get transaction: %w", err)
	}
	return t, nil
}

func (r *repository) Create(ctx context.Context, t *models.Transaction) error {
	const q = `
		INSERT INTO transactions (user_id, category_id, amount, currency, type, description, date)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at`

	return r.db.QueryRowContext(ctx, q,
		t.UserID, t.CategoryID, t.Amount, t.Currency, t.Type, t.Description, t.Date,
	).Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
}

func (r *repository) Update(ctx context.Context, t *models.Transaction) error {
	const q = `
		UPDATE transactions
		SET category_id=$1, amount=$2, currency=$3, type=$4, description=$5, date=$6
		WHERE id=$7 AND user_id=$8
		RETURNING updated_at`

	err := r.db.QueryRowContext(ctx, q,
		t.CategoryID, t.Amount, t.Currency, t.Type, t.Description, t.Date, t.ID, t.UserID,
	).Scan(&t.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (r *repository) Delete(ctx context.Context, id, userID int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM transactions WHERE id=$1 AND user_id=$2`, id, userID)
	if err != nil {
		return fmt.Errorf("delete transaction: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *repository) Balance(ctx context.Context, userID int64, currency string) (*models.BalanceResponse, error) {
	const q = `
		SELECT
			COALESCE(SUM(CASE WHEN type='income'  THEN amount ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN type='expense' THEN amount ELSE 0 END), 0)
		FROM transactions
		WHERE user_id=$1 AND currency=$2`

	b := &models.BalanceResponse{UserID: userID, Currency: currency}
	if err := r.db.QueryRowContext(ctx, q, userID, currency).Scan(&b.Income, &b.Expense); err != nil {
		return nil, fmt.Errorf("balance query: %w", err)
	}
	b.Balance = b.Income - b.Expense
	return b, nil
}

func (r *repository) AddAttachment(ctx context.Context, a *models.Attachment) error {
	const q = `
		INSERT INTO attachments (transaction_id, user_id, file_name, file_path, file_size, content_type)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at`

	return r.db.QueryRowContext(ctx, q,
		a.TransactionID, a.UserID, a.FileName, a.FilePath, a.FileSize, a.ContentType,
	).Scan(&a.ID, &a.CreatedAt)
}

func (r *repository) GetAttachmentByID(ctx context.Context, id, userID int64) (*models.Attachment, error) {
	const q = `
		SELECT id, transaction_id, user_id, file_name, file_path, file_size, content_type, created_at
		FROM attachments WHERE id=$1 AND user_id=$2`
	a := &models.Attachment{}
	err := r.db.QueryRowContext(ctx, q, id, userID).Scan(
		&a.ID, &a.TransactionID, &a.UserID,
		&a.FileName, &a.FilePath, &a.FileSize, &a.ContentType, &a.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return a, err
}

func (r *repository) ListAttachments(ctx context.Context, txID, userID int64) ([]models.Attachment, error) {
	const q = `
		SELECT id, transaction_id, user_id, file_name, file_path, file_size, content_type, created_at
		FROM attachments WHERE transaction_id=$1 AND user_id=$2`

	rows, err := r.db.QueryContext(ctx, q, txID, userID)
	if err != nil {
		return nil, fmt.Errorf("list attachments: %w", err)
	}
	defer rows.Close()

	var list []models.Attachment
	for rows.Next() {
		var a models.Attachment
		if err := rows.Scan(&a.ID, &a.TransactionID, &a.UserID,
			&a.FileName, &a.FilePath, &a.FileSize, &a.ContentType, &a.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, a)
	}
	return list, rows.Err()
}
