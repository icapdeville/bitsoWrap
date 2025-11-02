package bitso

type BalanceItem struct {
	Currency           string `json:"currency"`
	Total              string `json:"total"`
	Locked             string `json:"locked"`
	Available          string `json:"available"`
	PendingDeposit     string `json:"pending_deposit"`
	PendingWithdrawal  string `json:"pending_withdrawal"`
}

type BalancePayload struct {
	Balances []BalanceItem `json:"balances"`
}

type BitsoResponse struct {
	Success bool           `json:"success"`
	Payload BalancePayload `json:"payload"`
}


type BookFeeStructure struct {
	Volume string `json:"volume"`
	Maker  string `json:"maker"`
	Taker  string `json:"taker"`
}

type BookFees struct {
	FlatRate string              `json:"flat_rate"`
	Structure []BookFeeStructure `json:"structure"` 
}

type AvailableBook struct {
	Book           string   `json:"book"`
	MinimumAmount  string   `json:"minimum_amount"` 
	MaximumAmount  string   `json:"maximum_amount"`
	MinimumValue   string   `json:"minimum_value"`
	MaximumValue   string   `json:"maximum_value"`
	TickSize       string   `json:"tick_size"`  
	Fees           BookFees `json:"fees"`
	DefaultChart   string   `json:"default_chart"`
	MinimumPrice   string   `json:"minimum_price"`
	MaximumPrice   string   `json:"maximum_price"`
}

type AvailableBooksResponse struct {
	Success bool            `json:"success"`
	Payload []AvailableBook `json:"payload"`
}


type RollingAverageChange struct {
    SixHours string `json:"6"` 
}

type TickerPayload struct {
	Book      string `json:"book"`
	Volume    string `json:"volume"`
	High      string `json:"high"`
	Last      string `json:"last"` 
	Low       string `json:"low"`
	Vwap      string `json:"vwap"`
	Ask       string `json:"ask"` 
	Bid       string `json:"bid"` 
	CreatedAt string `json:"created_at"`
	Change24 string `json:"change_24"` 
    RollingAverageChange RollingAverageChange `json:"rolling_average_change"`
}

type TickerResponse struct {
	Success bool          `json:"success"`
	Payload TickerPayload `json:"payload"`
}


type BitsoOrderPayload struct {
	Oid string `json:"oid"` 
}

type BitsoOrderError struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details"` 
}

type BitsoOrderResponse struct {
	Success bool              `json:"success"`
	Payload *BitsoOrderPayload `json:"payload,omitempty"`
	Error   *BitsoOrderError   `json:"error,omitempty"`
}
