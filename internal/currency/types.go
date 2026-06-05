package currency

import "time"

type ConvertRequest struct {
	Amount       float64 `json:"amount"`
	FromCurrency string  `json:"from_currency"`
	ToCurrency   string  `json:"to_currency"`
}

type ConvertResponse struct {
	ConvertedAmount float64 `json:"converted_amount"`
	ExchangeRate    float64 `json:"exchange_rate"`
	FromCurrency    string  `json:"from_currency"`
	ToCurrency      string  `json:"to_currency"`
}

type GetRatesRequest struct {
	BaseCurrency string `json:"base_currency"`
}

type GetRatesResponse struct {
	BaseCurrency string             `json:"base_currency"`
	Rates        map[string]float64 `json:"rates"`
	UpdatedAt    time.Time          `json:"updated_at"`
}
