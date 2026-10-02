package api

import (
	"bitsoWrap/internal/api/handlers"
	"net/http"
)

func NewRouter() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/bal", handlers.BalanceHandler)
	mux.HandleFunc("/ticker", handlers.TickerHandler)
	mux.HandleFunc("/orders", handlers.PlaceOrderHandler)
	mux.HandleFunc("/user_trades", handlers.GetUserTradesHandler)
	mux.HandleFunc("/open_orders", handlers.GetOpenOrdersHandler)
	mux.HandleFunc("/fundings", handlers.GetFundingsHandler)
	mux.HandleFunc("/withdrawals", handlers.GetWithdrawalsHandler)

	return mux
}
