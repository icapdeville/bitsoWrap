package api

import (
    "net/http"
    "bitsoWrap/internal/api/handlers"
)

func NewRouter() *http.ServeMux {
    mux := http.NewServeMux()

    mux.HandleFunc("/bal", handlers.BalanceHandler)
    mux.HandleFunc("/ticker", handlers.TickerHandler)
	mux.HandleFunc("/orders", handlers.PlaceOrderHandler)
	mux.HandleFunc("/user_trades", handlers.GetUserTradesHandler)
	mux.HandleFunc("/open_orders", handlers.GetOpenOrdersHandler)

    return mux
}
