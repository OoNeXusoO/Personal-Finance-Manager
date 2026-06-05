package currency

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CurrencyServiceServer interface {
	Convert(ctx context.Context, req *ConvertRequest) (*ConvertResponse, error)
	GetRates(ctx context.Context, req *GetRatesRequest) (*GetRatesResponse, error)
}

var ServiceDesc = grpc.ServiceDesc{
	ServiceName: "currency.CurrencyService",
	HandlerType: (*CurrencyServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Convert",
			Handler:    convertHandler,
		},
		{
			MethodName: "GetRates",
			Handler:    getRatesHandler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "currency.proto",
}

func convertHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ConvertRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CurrencyServiceServer).Convert(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/currency.CurrencyService/Convert"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CurrencyServiceServer).Convert(ctx, req.(*ConvertRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func getRatesHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetRatesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CurrencyServiceServer).GetRates(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/currency.CurrencyService/GetRates"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CurrencyServiceServer).GetRates(ctx, req.(*GetRatesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var exchangeRates = map[string]float64{
	"PLN": 1.0000,
	"EUR": 0.2300,
	"USD": 0.2500,
	"GBP": 0.1980,
	"CHF": 0.2210,
	"CZK": 5.7800,
	"NOK": 2.6200,
	"SEK": 2.7400,
}

type currencyServer struct{}

func NewServer() CurrencyServiceServer { return &currencyServer{} }

func (s *currencyServer) Convert(_ context.Context, req *ConvertRequest) (*ConvertResponse, error) {
	if req.Amount <= 0 {
		return nil, status.Errorf(codes.InvalidArgument, "amount must be positive")
	}
	fromRate, ok := exchangeRates[req.FromCurrency]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "unsupported currency: %s", req.FromCurrency)
	}
	toRate, ok := exchangeRates[req.ToCurrency]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "unsupported currency: %s", req.ToCurrency)
	}

	inPLN := req.Amount / fromRate
	converted := inPLN * toRate
	exchangeRate := toRate / fromRate

	return &ConvertResponse{
		ConvertedAmount: roundTo(converted, 4),
		ExchangeRate:    roundTo(exchangeRate, 6),
		FromCurrency:    req.FromCurrency,
		ToCurrency:      req.ToCurrency,
	}, nil
}

func (s *currencyServer) GetRates(_ context.Context, req *GetRatesRequest) (*GetRatesResponse, error) {
	baseRate, ok := exchangeRates[req.BaseCurrency]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "unsupported base currency: %s", req.BaseCurrency)
	}

	rates := make(map[string]float64, len(exchangeRates))
	for code, rate := range exchangeRates {
		rates[code] = roundTo(rate/baseRate, 6)
	}

	return &GetRatesResponse{
		BaseCurrency: req.BaseCurrency,
		Rates:        rates,
		UpdatedAt:    time.Now(),
	}, nil
}

func roundTo(val float64, places int) float64 {
	shift := 1.0
	for i := 0; i < places; i++ {
		shift *= 10
	}
	return float64(int(val*shift+0.5)) / shift
}

func RegisterServer(s *grpc.Server, srv CurrencyServiceServer) {
	s.RegisterService(&ServiceDesc, srv)
}

var ErrUnsupportedCurrency = errors.New("unsupported currency")

func SupportedCurrencies() []string {
	out := make([]string, 0, len(exchangeRates))
	for c := range exchangeRates {
		out = append(out, c)
	}
	return out
}
