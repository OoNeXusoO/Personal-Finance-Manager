package models

import "time"

type User struct {
	ID           int64     `json:"id"         db:"id"`
	Email        string    `json:"email"      db:"email"`
	PasswordHash string    `json:"-"          db:"password_hash"`
	Name         string    `json:"name"       db:"name"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

type Category struct {
	ID        int64     `json:"id"         db:"id"`
	UserID    int64     `json:"user_id"    db:"user_id"`
	Name      string    `json:"name"       db:"name"`
	Type      string    `json:"type"       db:"type"`
	Color     string    `json:"color"      db:"color"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type Transaction struct {
	ID          int64     `json:"id"          db:"id"`
	UserID      int64     `json:"user_id"     db:"user_id"`
	CategoryID  *int64    `json:"category_id" db:"category_id"`
	Amount      float64   `json:"amount"      db:"amount"`
	Currency    string    `json:"currency"    db:"currency"`
	Type        string    `json:"type"        db:"type"`
	Description string    `json:"description" db:"description"`
	Date        time.Time `json:"date"        db:"date"`
	CreatedAt   time.Time `json:"created_at"  db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"  db:"updated_at"`
}

type Attachment struct {
	ID            int64     `json:"id"             db:"id"`
	TransactionID int64     `json:"transaction_id" db:"transaction_id"`
	UserID        int64     `json:"user_id"        db:"user_id"`
	FileName      string    `json:"file_name"      db:"file_name"`
	FilePath      string    `json:"file_path"      db:"file_path"`
	FileSize      int64     `json:"file_size"      db:"file_size"`
	ContentType   string    `json:"content_type"   db:"content_type"`
	CreatedAt     time.Time `json:"created_at"     db:"created_at"`
}

type ApiLog struct {
	ID         int64     `json:"id"          db:"id"`
	UserID     *int64    `json:"user_id"     db:"user_id"`
	Method     string    `json:"method"      db:"method"`
	Path       string    `json:"path"        db:"path"`
	StatusCode int       `json:"status_code" db:"status_code"`
	DurationMs int64     `json:"duration_ms" db:"duration_ms"`
	UserAgent  string    `json:"user_agent"  db:"user_agent"`
	IPAddress  string    `json:"ip_address"  db:"ip_address"`
	CreatedAt  time.Time `json:"created_at"  db:"created_at"`
}

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

func (r *RegisterRequest) Validate() error {
	if r.Email == "" {
		return ErrValidation("email is required")
	}
	if len(r.Password) < 8 {
		return ErrValidation("password must be at least 8 characters")
	}
	if r.Name == "" {
		return ErrValidation("name is required")
	}
	return nil
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type CreateCategoryRequest struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Color string `json:"color"`
}

func (r *CreateCategoryRequest) Validate() error {
	if r.Name == "" {
		return ErrValidation("name is required")
	}
	if r.Type != "income" && r.Type != "expense" {
		return ErrValidation("type must be 'income' or 'expense'")
	}
	return nil
}

type CreateTransactionRequest struct {
	CategoryID  *int64  `json:"category_id"`
	Amount      float64 `json:"amount"`
	Currency    string  `json:"currency"`
	Type        string  `json:"type"`
	Description string  `json:"description"`
	Date        string  `json:"date"`
}

func (r *CreateTransactionRequest) Validate() error {
	if r.Amount <= 0 {
		return ErrValidation("amount must be greater than zero")
	}
	if r.Type != "income" && r.Type != "expense" {
		return ErrValidation("type must be 'income' or 'expense'")
	}
	if r.Currency == "" {
		r.Currency = "PLN"
	}
	if len(r.Currency) != 3 {
		return ErrValidation("currency must be a 3-letter ISO code (e.g. PLN)")
	}
	return nil
}

type UpdateTransactionRequest struct {
	CategoryID  *int64  `json:"category_id"`
	Amount      float64 `json:"amount"`
	Currency    string  `json:"currency"`
	Type        string  `json:"type"`
	Description string  `json:"description"`
	Date        string  `json:"date"`
}

func (r *UpdateTransactionRequest) Validate() error {
	if r.Amount <= 0 {
		return ErrValidation("amount must be greater than zero")
	}
	if r.Type != "income" && r.Type != "expense" {
		return ErrValidation("type must be 'income' or 'expense'")
	}
	return nil
}

type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type BalanceResponse struct {
	UserID   int64   `json:"user_id"`
	Balance  float64 `json:"balance"`
	Income   float64 `json:"income"`
	Expense  float64 `json:"expense"`
	Currency string  `json:"currency"`
}

type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

const (
	WSTypeBalanceUpdate = "balance_update"
	WSTypePing          = "ping"
	WSTypePong          = "pong"
)

type ValidationError string

func (e ValidationError) Error() string { return string(e) }

func ErrValidation(msg string) ValidationError { return ValidationError(msg) }
