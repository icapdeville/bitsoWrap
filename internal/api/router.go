package api

import (
	"bitsoWrap/internal/api/handlers"
	"net/http"
)

func NewRouter() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", handlers.HealthHandler)
	mux.HandleFunc("/bal", handlers.BalanceHandler)
	mux.HandleFunc("/ticker", handlers.TickerHandler)
	mux.HandleFunc("/orders", handlers.PlaceOrderHandler)
	mux.HandleFunc("/user_trades", handlers.GetUserTradesHandler)
	mux.HandleFunc("/open_orders", handlers.GetOpenOrdersHandler)
	mux.HandleFunc("/fundings", handlers.GetFundingsHandler)
	mux.HandleFunc("/withdrawals", handlers.GetWithdrawalsHandler)
	mux.HandleFunc("/saldo", handlers.SaldoHandler)
	mux.HandleFunc("/saldos", handlers.SaldosHandler)
	mux.HandleFunc("/saldo/historial", handlers.SaldoHistorialHandler)
	mux.HandleFunc("GET /wallets/{wallet}", handlers.GetWalletHandler)
	mux.HandleFunc("PUT /wallets/{wallet}", handlers.PutWalletHandler)
	mux.HandleFunc("GET /wallets/{wallet}/historial", handlers.WalletHistorialHandler)

	return mux
}
