package tests

import (
	"context"
	"testing"
	"time"

	"personal-finance-manager/internal/auth"
	"personal-finance-manager/internal/currency"
	"personal-finance-manager/internal/models"
)

type mockAuthRepo struct {
	users  map[string]*models.User
	nextID int64
}

func newMockAuthRepo() *mockAuthRepo {
	return &mockAuthRepo{users: make(map[string]*models.User), nextID: 1}
}

func (m *mockAuthRepo) Create(_ context.Context, u *models.User) error {
	if _, exists := m.users[u.Email]; exists {
		return auth.ErrEmailExists
	}
	u.ID = m.nextID
	m.nextID++
	u.CreatedAt = time.Now()
	u.UpdatedAt = time.Now()
	m.users[u.Email] = u
	return nil
}

func (m *mockAuthRepo) FindByEmail(_ context.Context, email string) (*models.User, error) {
	u, ok := m.users[email]
	if !ok {
		return nil, auth.ErrNotFound
	}
	return u, nil
}

func (m *mockAuthRepo) FindByID(_ context.Context, id int64) (*models.User, error) {
	for _, u := range m.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, auth.ErrNotFound
}

func TestRegister_Success(t *testing.T) {
	svc := auth.NewService(newMockAuthRepo(), "test-secret-key-32chars-minimum!!", 24*time.Hour)

	user, err := svc.Register(context.Background(), &models.RegisterRequest{
		Email:    "test@example.com",
		Password: "password123",
		Name:     "Test User",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if user.ID == 0 {
		t.Error("expected non-zero user ID")
	}
	if user.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", user.Email)
	}
	if user.PasswordHash != "" && user.PasswordHash == "password123" {
		t.Error("password must be hashed, not stored in plain text")
	}
}

func TestRegister_ShortPassword(t *testing.T) {
	svc := auth.NewService(newMockAuthRepo(), "test-secret-key-32chars-minimum!!", 24*time.Hour)

	_, err := svc.Register(context.Background(), &models.RegisterRequest{
		Email:    "test@example.com",
		Password: "short",
		Name:     "Test",
	})
	if err == nil {
		t.Fatal("expected validation error for short password")
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	repo := newMockAuthRepo()
	svc := auth.NewService(repo, "test-secret-key-32chars-minimum!!", 24*time.Hour)

	req := &models.RegisterRequest{Email: "dup@example.com", Password: "password123", Name: "A"}
	if _, err := svc.Register(context.Background(), req); err != nil {
		t.Fatalf("first registration failed: %v", err)
	}
	_, err := svc.Register(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for duplicate email")
	}
}

func TestLogin_InvalidCredentials(t *testing.T) {
	svc := auth.NewService(newMockAuthRepo(), "test-secret-key-32chars-minimum!!", 24*time.Hour)

	_, err := svc.Login(context.Background(), &models.LoginRequest{
		Email:    "nobody@example.com",
		Password: "password123",
	})
	if err == nil {
		t.Fatal("expected error for unknown user")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	repo := newMockAuthRepo()
	svc := auth.NewService(repo, "test-secret-key-32chars-minimum!!", 24*time.Hour)

	_, _ = svc.Register(context.Background(), &models.RegisterRequest{
		Email: "user@example.com", Password: "correct-password", Name: "U",
	})

	_, err := svc.Login(context.Background(), &models.LoginRequest{
		Email:    "user@example.com",
		Password: "wrong-password",
	})
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
}

func TestTokenRoundtrip(t *testing.T) {
	secret := "test-secret-key-32chars-minimum!!"
	repo := newMockAuthRepo()
	svc := auth.NewService(repo, secret, 24*time.Hour)

	_, _ = svc.Register(context.Background(), &models.RegisterRequest{
		Email: "tok@example.com", Password: "password123", Name: "Tok",
	})

	resp, err := svc.Login(context.Background(), &models.LoginRequest{
		Email: "tok@example.com", Password: "password123",
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	claims, err := svc.ValidateToken(resp.Token)
	if err != nil {
		t.Fatalf("token validation failed: %v", err)
	}
	if claims.Email != "tok@example.com" {
		t.Errorf("expected email tok@example.com, got %s", claims.Email)
	}
}

func TestTransactionValidation_NegativeAmount(t *testing.T) {
	req := &models.CreateTransactionRequest{
		Amount: -50.0,
		Type:   "expense",
	}
	if err := req.Validate(); err == nil {
		t.Error("expected validation error for negative amount")
	}
}

func TestTransactionValidation_ZeroAmount(t *testing.T) {
	req := &models.CreateTransactionRequest{
		Amount: 0,
		Type:   "income",
	}
	if err := req.Validate(); err == nil {
		t.Error("expected validation error for zero amount")
	}
}

func TestTransactionValidation_InvalidType(t *testing.T) {
	req := &models.CreateTransactionRequest{
		Amount: 100,
		Type:   "transfer",
	}
	if err := req.Validate(); err == nil {
		t.Error("expected validation error for invalid type")
	}
}

func TestTransactionValidation_ValidIncome(t *testing.T) {
	req := &models.CreateTransactionRequest{
		Amount:   1500.50,
		Type:     "income",
		Currency: "PLN",
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected no error for valid income: %v", err)
	}
}

func TestTransactionValidation_InvalidCurrency(t *testing.T) {
	req := &models.CreateTransactionRequest{
		Amount:   100,
		Type:     "expense",
		Currency: "EURO",
	}
	if err := req.Validate(); err == nil {
		t.Error("expected validation error for invalid currency code")
	}
}

func TestCurrencyServer_Convert(t *testing.T) {
	srv := currency.NewServer()

	resp, err := srv.Convert(context.Background(), &currency.ConvertRequest{
		Amount:       100,
		FromCurrency: "PLN",
		ToCurrency:   "EUR",
	})
	if err != nil {
		t.Fatalf("convert PLN→EUR failed: %v", err)
	}
	if resp.ConvertedAmount <= 0 {
		t.Errorf("expected positive converted amount, got %f", resp.ConvertedAmount)
	}
	if resp.ExchangeRate <= 0 {
		t.Errorf("expected positive exchange rate, got %f", resp.ExchangeRate)
	}
}

func TestCurrencyServer_Convert_SameCurrency(t *testing.T) {
	srv := currency.NewServer()

	resp, err := srv.Convert(context.Background(), &currency.ConvertRequest{
		Amount:       250,
		FromCurrency: "PLN",
		ToCurrency:   "PLN",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ConvertedAmount != 250 {
		t.Errorf("same-currency conversion should return same amount, got %f", resp.ConvertedAmount)
	}
}

func TestCurrencyServer_Convert_UnknownCurrency(t *testing.T) {
	srv := currency.NewServer()

	_, err := srv.Convert(context.Background(), &currency.ConvertRequest{
		Amount:       100,
		FromCurrency: "PLN",
		ToCurrency:   "XYZ",
	})
	if err == nil {
		t.Error("expected error for unknown currency")
	}
}

func TestCurrencyServer_Convert_NegativeAmount(t *testing.T) {
	srv := currency.NewServer()

	_, err := srv.Convert(context.Background(), &currency.ConvertRequest{
		Amount:       -1,
		FromCurrency: "PLN",
		ToCurrency:   "EUR",
	})
	if err == nil {
		t.Error("expected error for negative amount")
	}
}

func TestCurrencyServer_GetRates(t *testing.T) {
	srv := currency.NewServer()

	resp, err := srv.GetRates(context.Background(), &currency.GetRatesRequest{BaseCurrency: "PLN"})
	if err != nil {
		t.Fatalf("GetRates failed: %v", err)
	}
	if len(resp.Rates) == 0 {
		t.Error("expected non-empty rates map")
	}
	if resp.Rates["PLN"] != 1.0 {
		t.Errorf("PLN rate relative to PLN should be 1.0, got %f", resp.Rates["PLN"])
	}
}

func TestParseDate(t *testing.T) {
	cases := []string{"2026-05-28", "2026-01-01", "invalid", "28-05-2026"}

	for _, input := range cases {
		req := &models.CreateTransactionRequest{
			Amount: 100,
			Type:   "income",
			Date:   input,
		}
		if err := req.Validate(); err != nil {
			t.Errorf("Validate() with date=%q returned unexpected error: %v", input, err)
		}
	}
}
