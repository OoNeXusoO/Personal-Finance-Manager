package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"personal-finance-manager/internal/auth"
	"personal-finance-manager/internal/categories"
	"personal-finance-manager/internal/models"
	"personal-finance-manager/internal/transactions"
	"personal-finance-manager/pkg/response"
)

const testJWTSecret = "test-secret-key-32chars-minimum!!"

type apiEnv struct {
	router  http.Handler
	authSvc auth.Service
}

func newTestEnv(t *testing.T) *apiEnv {
	t.Helper()

	repo := newMockAuthRepo()
	authSvc := auth.NewService(repo, testJWTSecret, 24*time.Hour)
	authHandler := auth.NewHandler(authSvc)
	jwtMW := auth.Middleware(authSvc)

	catRepo := &mockCatRepo{cats: make(map[int64]*models.Category), nextID: 1}
	catSvc := categories.NewService(catRepo)
	catHandler := categories.NewHandler(catSvc)

	txRepo := &mockTxRepo{txs: make(map[int64]*models.Transaction), nextID: 1}
	txSvc := transactions.NewService(txRepo, nil)
	txHandler := transactions.NewHandler(txSvc, txRepo, t.TempDir(), 10<<20)

	r := chi.NewRouter()
	r.Post("/auth/register", authHandler.Register)
	r.Post("/auth/login", authHandler.Login)

	r.Group(func(r chi.Router) {
		r.Use(jwtMW)
		r.Get("/transactions", txHandler.List)
		r.Post("/transactions", txHandler.Create)
		r.Put("/transactions/{id}", txHandler.Update)
		r.Delete("/transactions/{id}", txHandler.Delete)
		r.Get("/balance", txHandler.GetBalance)
		r.Get("/categories", catHandler.List)
		r.Post("/categories", catHandler.Create)
		r.Delete("/categories/{id}", catHandler.Delete)
	})

	return &apiEnv{router: r, authSvc: authSvc}
}

func (e *apiEnv) do(method, path string, body interface{}, token string) *httptest.ResponseRecorder {
	var b bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&b).Encode(body)
	}
	req := httptest.NewRequest(method, path, &b)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	e.router.ServeHTTP(rr, req)
	return rr
}

func mustJSON(t *testing.T, rr *httptest.ResponseRecorder, out interface{}) {
	t.Helper()
	if err := json.NewDecoder(rr.Body).Decode(out); err != nil {
		t.Fatalf("decode response body: %v (body: %s)", err, rr.Body.String())
	}
}

func registerAndLogin(t *testing.T, e *apiEnv, email, password string) string {
	t.Helper()
	e.do("POST", "/auth/register", models.RegisterRequest{
		Email: email, Password: password, Name: "Test",
	}, "")

	rr := e.do("POST", "/auth/login", models.LoginRequest{
		Email: email, Password: password,
	}, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("login failed: status %d, body: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Data models.LoginResponse `json:"data"`
	}
	mustJSON(t, rr, &resp)
	return resp.Data.Token
}

func TestIntegration_Register(t *testing.T) {
	e := newTestEnv(t)

	rr := e.do("POST", "/auth/register", models.RegisterRequest{
		Email: "int@example.com", Password: "password123", Name: "Integration",
	}, "")

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestIntegration_Register_DuplicateEmail(t *testing.T) {
	e := newTestEnv(t)
	body := models.RegisterRequest{Email: "dup@example.com", Password: "password123", Name: "A"}

	e.do("POST", "/auth/register", body, "")
	rr := e.do("POST", "/auth/register", body, "")

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rr.Code)
	}
}

func TestIntegration_Login_InvalidPassword(t *testing.T) {
	e := newTestEnv(t)
	e.do("POST", "/auth/register", models.RegisterRequest{
		Email: "x@example.com", Password: "password123", Name: "X",
	}, "")

	rr := e.do("POST", "/auth/login", models.LoginRequest{
		Email: "x@example.com", Password: "wrong",
	}, "")

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestIntegration_CreateTransaction(t *testing.T) {
	e := newTestEnv(t)
	token := registerAndLogin(t, e, "tx@example.com", "password123")

	rr := e.do("POST", "/transactions", models.CreateTransactionRequest{
		Amount: 500.0, Type: "income", Currency: "PLN", Description: "Test income",
	}, token)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Data models.Transaction `json:"data"`
	}
	mustJSON(t, rr, &resp)
	if resp.Data.Amount != 500.0 {
		t.Errorf("expected amount 500, got %f", resp.Data.Amount)
	}
}

func TestIntegration_CreateTransaction_Unauthorized(t *testing.T) {
	e := newTestEnv(t)

	rr := e.do("POST", "/transactions", models.CreateTransactionRequest{
		Amount: 100, Type: "income",
	}, "")

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestIntegration_CreateTransaction_InvalidAmount(t *testing.T) {
	e := newTestEnv(t)
	token := registerAndLogin(t, e, "inv@example.com", "password123")

	rr := e.do("POST", "/transactions", models.CreateTransactionRequest{
		Amount: -100, Type: "income",
	}, token)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestIntegration_ListTransactions(t *testing.T) {
	e := newTestEnv(t)
	token := registerAndLogin(t, e, "list@example.com", "password123")

	for i := 0; i < 3; i++ {
		e.do("POST", "/transactions", models.CreateTransactionRequest{
			Amount: 100, Type: "income", Currency: "PLN",
		}, token)
	}

	rr := e.do("GET", "/transactions", nil, token)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp struct {
		Data []models.Transaction `json:"data"`
		Meta response.Meta        `json:"meta"`
	}
	mustJSON(t, rr, &resp)

	if len(resp.Data) != 3 {
		t.Errorf("expected 3 transactions, got %d", len(resp.Data))
	}
	if resp.Meta.Total != 3 {
		t.Errorf("expected meta.total=3, got %d", resp.Meta.Total)
	}
}

func TestIntegration_DeleteTransaction_NotOwner(t *testing.T) {
	e := newTestEnv(t)
	token1 := registerAndLogin(t, e, "owner@example.com", "password123")
	token2 := registerAndLogin(t, e, "other@example.com", "password123")

	rr := e.do("POST", "/transactions", models.CreateTransactionRequest{
		Amount: 200, Type: "expense", Currency: "PLN",
	}, token1)

	var created struct {
		Data models.Transaction `json:"data"`
	}
	mustJSON(t, rr, &created)

	del := e.do("DELETE", "/transactions/"+itoa(created.Data.ID), nil, token2)
	if del.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when deleting other user's transaction, got %d", del.Code)
	}
}

func TestIntegration_Balance(t *testing.T) {
	e := newTestEnv(t)
	token := registerAndLogin(t, e, "bal@example.com", "password123")

	e.do("POST", "/transactions", models.CreateTransactionRequest{
		Amount: 1000, Type: "income", Currency: "PLN",
	}, token)
	e.do("POST", "/transactions", models.CreateTransactionRequest{
		Amount: 300, Type: "expense", Currency: "PLN",
	}, token)

	rr := e.do("GET", "/balance?currency=PLN", nil, token)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp struct {
		Data models.BalanceResponse `json:"data"`
	}
	mustJSON(t, rr, &resp)

	if resp.Data.Income != 1000 {
		t.Errorf("expected income=1000, got %f", resp.Data.Income)
	}
	if resp.Data.Expense != 300 {
		t.Errorf("expected expense=300, got %f", resp.Data.Expense)
	}
	if resp.Data.Balance != 700 {
		t.Errorf("expected balance=700, got %f", resp.Data.Balance)
	}
}

type mockCatRepo struct {
	cats   map[int64]*models.Category
	nextID int64
}

func (m *mockCatRepo) List(_ context.Context, userID int64) ([]models.Category, error) {
	var out []models.Category
	for _, c := range m.cats {
		if c.UserID == userID {
			out = append(out, *c)
		}
	}
	return out, nil
}

func (m *mockCatRepo) GetByID(_ context.Context, id, userID int64) (*models.Category, error) {
	c, ok := m.cats[id]
	if !ok || c.UserID != userID {
		return nil, categories.ErrNotFound
	}
	return c, nil
}

func (m *mockCatRepo) Create(_ context.Context, c *models.Category) error {
	c.ID = m.nextID
	m.nextID++
	c.CreatedAt = time.Now()
	m.cats[c.ID] = c
	return nil
}

func (m *mockCatRepo) Delete(_ context.Context, id, userID int64) error {
	c, ok := m.cats[id]
	if !ok || c.UserID != userID {
		return categories.ErrNotFound
	}
	delete(m.cats, id)
	return nil
}

type mockTxRepo struct {
	txs         map[int64]*models.Transaction
	nextID      int64
	attachments []models.Attachment
}

func (m *mockTxRepo) List(_ context.Context, userID int64, f transactions.Filter) ([]models.Transaction, int, error) {
	var out []models.Transaction
	for _, t := range m.txs {
		if t.UserID != userID {
			continue
		}
		if f.Type != "" && t.Type != f.Type {
			continue
		}
		out = append(out, *t)
	}
	total := len(out)
	start := (f.Page - 1) * f.PerPage
	if start >= total {
		return []models.Transaction{}, total, nil
	}
	end := start + f.PerPage
	if end > total {
		end = total
	}
	return out[start:end], total, nil
}

func (m *mockTxRepo) GetByID(_ context.Context, id, userID int64) (*models.Transaction, error) {
	t, ok := m.txs[id]
	if !ok || t.UserID != userID {
		return nil, transactions.ErrNotFound
	}
	return t, nil
}

func (m *mockTxRepo) Create(_ context.Context, t *models.Transaction) error {
	t.ID = m.nextID
	m.nextID++
	t.CreatedAt = time.Now()
	t.UpdatedAt = time.Now()
	if t.Date.IsZero() {
		t.Date = time.Now()
	}
	cp := *t
	m.txs[t.ID] = &cp
	return nil
}

func (m *mockTxRepo) Update(_ context.Context, t *models.Transaction) error {
	existing, ok := m.txs[t.ID]
	if !ok || existing.UserID != t.UserID {
		return transactions.ErrNotFound
	}
	t.UpdatedAt = time.Now()
	cp := *t
	m.txs[t.ID] = &cp
	return nil
}

func (m *mockTxRepo) Delete(_ context.Context, id, userID int64) error {
	t, ok := m.txs[id]
	if !ok || t.UserID != userID {
		return transactions.ErrNotFound
	}
	delete(m.txs, id)
	return nil
}

func (m *mockTxRepo) Balance(_ context.Context, userID int64, currency string) (*models.BalanceResponse, error) {
	b := &models.BalanceResponse{UserID: userID, Currency: currency}
	for _, t := range m.txs {
		if t.UserID != userID || t.Currency != currency {
			continue
		}
		if t.Type == "income" {
			b.Income += t.Amount
		} else {
			b.Expense += t.Amount
		}
	}
	b.Balance = b.Income - b.Expense
	return b, nil
}

func (m *mockTxRepo) GetAttachmentByID(_ context.Context, id, userID int64) (*models.Attachment, error) {
	for _, a := range m.attachments {
		if a.ID == id && a.UserID == userID {
			return &a, nil
		}
	}
	return nil, transactions.ErrNotFound
}

func (m *mockTxRepo) AddAttachment(_ context.Context, a *models.Attachment) error {
	a.ID = int64(len(m.attachments) + 1)
	a.CreatedAt = time.Now()
	m.attachments = append(m.attachments, *a)
	return nil
}

func (m *mockTxRepo) ListAttachments(_ context.Context, txID, userID int64) ([]models.Attachment, error) {
	var out []models.Attachment
	for _, a := range m.attachments {
		if a.TransactionID == txID && a.UserID == userID {
			out = append(out, a)
		}
	}
	return out, nil
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 20)
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}
