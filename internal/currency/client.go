package currency

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	conn *grpc.ClientConn
	addr string
}

func NewClient(addr string) (*Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("dial currency service at %s: %w", addr, err)
	}
	return &Client{conn: conn, addr: addr}, nil
}

func (c *Client) Close() error { return c.conn.Close() }

func (c *Client) Convert(ctx context.Context, req *ConvertRequest) (*ConvertResponse, error) {
	resp := new(ConvertResponse)
	err := c.conn.Invoke(ctx, "/currency.CurrencyService/Convert", req, resp)
	if err != nil {
		return nil, fmt.Errorf("currency.Convert: %w", err)
	}
	return resp, nil
}

func (c *Client) GetRates(ctx context.Context, baseCurrency string) (*GetRatesResponse, error) {
	req := &GetRatesRequest{BaseCurrency: baseCurrency}
	resp := new(GetRatesResponse)
	err := c.conn.Invoke(ctx, "/currency.CurrencyService/GetRates", req, resp)
	if err != nil {
		return nil, fmt.Errorf("currency.GetRates: %w", err)
	}
	return resp, nil
}
