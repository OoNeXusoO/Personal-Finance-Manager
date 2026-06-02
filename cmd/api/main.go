package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"personal-finance-manager/internal/auth"
	"personal-finance-manager/internal/categories"
	"personal-finance-manager/internal/config"
	"personal-finance-manager/internal/currency"
	"personal-finance-manager/internal/database"
	"personal-finance-manager/internal/models"
	"personal-finance-manager/internal/transactions"
	ws "personal-finance-manager/internal/websocket"
	apimw "personal-finance-manager/pkg/middleware"
	"personal-finance-manager/pkg/response"
)

func main() {
	cfg := config.Load()

	db, err := database.NewPostgres(cfg)
	if err != nil {
		log.Fatalf("connect to postgres: %v", err)
	}
	defer db.Close()
	log.Println("connected to PostgreSQL")

	var currencyClient *currency.Client
	currencyClient, err = currency.NewClient(cfg.CurrencyGRPCAddr)
	if err != nil {
		log.Printf("warn: currency gRPC client unavailable (%v) – conversion endpoints disabled", err)
	} else {
		defer currencyClient.Close()
		log.Printf("connected to currency gRPC service at %s", cfg.CurrencyGRPCAddr)
	}

	hub := ws.NewHub()
	go hub.Run()

	authRepo := auth.NewRepository(db)
	authSvc := auth.NewService(authRepo, cfg.JWTSecret, cfg.JWTExpiration)
	authHandler := auth.NewHandler(authSvc)
	jwtMiddleware := auth.Middleware(authSvc)

	catRepo := categories.NewRepository(db)
	catSvc := categories.NewService(catRepo)
	catHandler := categories.NewHandler(catSvc)

	onBalanceChange := func(ctx context.Context, userID int64, cur string) {
		txRepo := transactions.NewRepository(db)
		b, err := txRepo.Balance(ctx, userID, cur)
		if err != nil {
			return
		}
		hub.BroadcastToUser(userID, models.WSMessage{
			Type:    models.WSTypeBalanceUpdate,
			Payload: b,
		})
	}

	txRepo := transactions.NewRepository(db)
	txSvc := transactions.NewService(txRepo, onBalanceChange)
	txHandler := transactions.NewHandler(txSvc, txRepo, cfg.UploadDir, cfg.MaxUploadSize)

	wsHandler := ws.NewHandler(hub)

	r := chi.NewRouter()

	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.Timeout(30 * time.Second))
	r.Use(apimw.CORS)
	r.Use(apimw.APILogger(db))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Post("/auth/register", authHandler.Register)
	r.Post("/auth/login", authHandler.Login)

	r.Group(func(r chi.Router) {
		r.Use(jwtMiddleware)

		r.Get("/transactions", txHandler.List)
		r.Post("/transactions", txHandler.Create)
		r.Put("/transactions/{id}", txHandler.Update)
		r.Delete("/transactions/{id}", txHandler.Delete)
		r.Post("/transactions/{id}/upload", txHandler.UploadReceipt)
		r.Get("/transactions/{id}/attachments", txHandler.GetAttachments)
		r.Get("/attachments/{attachmentId}/download", txHandler.DownloadAttachment)
		r.Get("/balance", txHandler.GetBalance)

		r.Get("/categories", catHandler.List)
		r.Post("/categories", catHandler.Create)
		r.Delete("/categories/{id}", catHandler.Delete)

		r.Get("/ws/balance", wsHandler.ServeWS)

		r.Get("/currency/rates", makeCurrencyRatesHandler(currencyClient))
		r.Get("/currency/convert", makeCurrencyConvertHandler(currencyClient))
	})

	addr := fmt.Sprintf("%s:%s", cfg.ServerHost, cfg.ServerPort)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("API server listening on %s", addr)
		if cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
			if err := srv.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile); err != nil && err != http.ErrServerClosed {
				log.Fatalf("HTTPS server error: %v", err)
			}
		} else {
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("HTTP server error: %v", err)
			}
		}
	}()

	<-quit
	log.Println("shutting down server…")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}
	log.Println("server stopped")
}

func makeCurrencyRatesHandler(client *currency.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if client == nil {
			response.Error(w, http.StatusServiceUnavailable, "currency service unavailable")
			return
		}
		base := r.URL.Query().Get("base")
		if base == "" {
			base = "PLN"
		}
		rates, err := client.GetRates(r.Context(), base)
		if err != nil {
			response.Error(w, http.StatusBadGateway, "currency service error")
			return
		}
		response.JSON(w, http.StatusOK, rates)
	}
}

func makeCurrencyConvertHandler(client *currency.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if client == nil {
			response.Error(w, http.StatusServiceUnavailable, "currency service unavailable")
			return
		}
		q := r.URL.Query()
		var req currency.ConvertRequest
		if _, err := fmt.Sscanf(q.Get("amount"), "%f", &req.Amount); err != nil {
			response.Error(w, http.StatusBadRequest, "amount must be a number")
			return
		}
		req.FromCurrency = q.Get("from")
		req.ToCurrency = q.Get("to")

		resp, err := client.Convert(r.Context(), &req)
		if err != nil {
			response.Error(w, http.StatusBadGateway, "conversion failed")
			return
		}
		response.JSON(w, http.StatusOK, resp)
	}
}
