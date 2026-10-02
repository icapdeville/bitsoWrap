package bitso

import "encoding/json"

// BitsoError es el objeto "error" que Bitso devuelve cuando success=false.
type BitsoError struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// BitsoOrderError se mantiene por compatibilidad.
type BitsoOrderError = BitsoError

// apiEnvelope contiene los campos comunes de toda respuesta de Bitso.
type apiEnvelope struct {
	Success bool        `json:"success"`
	Error   *BitsoError `json:"error,omitempty"`
}

func (e *apiEnvelope) ok() bool         { return e.Success }
func (e *apiEnvelope) err() *BitsoError { return e.Error }

// FlexString acepta un valor JSON string o numérico y siempre se serializa como string.
type FlexString string

func (f *FlexString) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		return nil
	}
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*f = FlexString(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	*f = FlexString(n.String())
	return nil
}

type envelope interface {
	ok() bool
	err() *BitsoError
}

type BalanceItem struct {
	Currency          string `json:"currency"`
	Total             string `json:"total"`
	Locked            string `json:"locked"`
	Available         string `json:"available"`
	PendingDeposit    string `json:"pending_deposit"`
	PendingWithdrawal string `json:"pending_withdrawal"`
}

type BalancePayload struct {
	Balances []BalanceItem `json:"balances"`
}

type BitsoResponse struct {
	apiEnvelope
	Payload BalancePayload `json:"payload"`
}

type BookFeeStructure struct {
	Volume string `json:"volume"`
	Maker  string `json:"maker"`
	Taker  string `json:"taker"`
}

type BookFees struct {
	FlatRate  string             `json:"flat_rate"`
	Structure []BookFeeStructure `json:"structure"`
}

type AvailableBook struct {
	Book          string   `json:"book"`
	MinimumAmount string   `json:"minimum_amount"`
	MaximumAmount string   `json:"maximum_amount"`
	MinimumValue  string   `json:"minimum_value"`
	MaximumValue  string   `json:"maximum_value"`
	TickSize      string   `json:"tick_size"`
	Fees          BookFees `json:"fees"`
	DefaultChart  string   `json:"default_chart"`
	MinimumPrice  string   `json:"minimum_price"`
	MaximumPrice  string   `json:"maximum_price"`
}

type AvailableBooksResponse struct {
	apiEnvelope
	Payload []AvailableBook `json:"payload"`
}

type RollingAverageChange struct {
	SixHours string `json:"6"`
}

type TickerPayload struct {
	Book                 string               `json:"book"`
	Volume               string               `json:"volume"`
	High                 string               `json:"high"`
	Last                 string               `json:"last"`
	Low                  string               `json:"low"`
	Vwap                 string               `json:"vwap"`
	Ask                  string               `json:"ask"`
	Bid                  string               `json:"bid"`
	CreatedAt            string               `json:"created_at"`
	Change24             string               `json:"change_24"`
	RollingAverageChange RollingAverageChange `json:"rolling_average_change"`
}

type TickerResponse struct {
	apiEnvelope
	Payload TickerPayload `json:"payload"`
}

// TickersResponse es la respuesta de /ticker sin "book": todos los libros.
type TickersResponse struct {
	apiEnvelope
	Payload []TickerPayload `json:"payload"`
}

type BitsoOrderPayload struct {
	Oid string `json:"oid"`
}

type BitsoOrderResponse struct {
	apiEnvelope
	Payload *BitsoOrderPayload `json:"payload,omitempty"`
}

type UserTradesResponse struct {
	apiEnvelope
	Payload []UserTrade `json:"payload"`
}

type UserTrade struct {
	Book            string     `json:"book"`
	Major           string     `json:"major"`
	Minor           string     `json:"minor"`
	MajorCurrency   string     `json:"major_currency"`
	MinorCurrency   string     `json:"minor_currency"`
	Price           string     `json:"price"`
	Side            string     `json:"side"`
	MakerSide       string     `json:"maker_side"`
	FeesCurrency    string     `json:"fees_currency"`
	FeesAmount      string     `json:"fees_amount"`
	Tid             FlexString `json:"tid"`
	Oid             string     `json:"oid"`
	CreatedAt       string     `json:"created_at"`
	OriginID        string     `json:"origin_id"`
	MarginOrderType string     `json:"margin_order_type,omitempty"`
}

// OpenOrder representa una orden abierta devuelta por /open_orders
type OpenOrder struct {
	Book           string  `json:"book"`
	CreatedAt      string  `json:"created_at"`
	Oid            string  `json:"oid"`
	OriginID       *string `json:"origin_id"`
	OriginalAmount string  `json:"original_amount"`
	OriginalValue  string  `json:"original_value"`
	Price          string  `json:"price"`
	Side           string  `json:"side"`
	Status         string  `json:"status"`
	TimeInForce    string  `json:"time_in_force"`
	Type           string  `json:"type"`
	UnfilledAmount string  `json:"unfilled_amount"`
	UpdatedAt      *string `json:"updated_at"`
}

// OpenOrdersResponse estructura tipada para /open_orders
type OpenOrdersResponse struct {
	apiEnvelope
	Payload []OpenOrder `json:"payload"`
}

// Funding es un depósito devuelto por /fundings.
// Details varía según el método (SPEI, cripto, etc.), por eso se guarda crudo.
type Funding struct {
	Fid                  string          `json:"fid"`
	Status               string          `json:"status"`
	CreatedAt            string          `json:"created_at"`
	Currency             string          `json:"currency"`
	Method               string          `json:"method"`
	MethodName           string          `json:"method_name"`
	Amount               string          `json:"amount"`
	Fee                  string          `json:"fee,omitempty"`
	Asset                string          `json:"asset,omitempty"`
	Network              string          `json:"network,omitempty"`
	Protocol             string          `json:"protocol,omitempty"`
	Integration          string          `json:"integration,omitempty"`
	Details              json.RawMessage `json:"details,omitempty"`
	LegalOperationEntity json.RawMessage `json:"legal_operation_entity,omitempty"`
}

type FundingsResponse struct {
	apiEnvelope
	Payload []Funding `json:"payload"`
}

// Withdrawal es un retiro devuelto por /withdrawals.
type Withdrawal struct {
	Wid                  string          `json:"wid"`
	Status               string          `json:"status"`
	CreatedAt            string          `json:"created_at"`
	Currency             string          `json:"currency"`
	Method               string          `json:"method"`
	MethodName           string          `json:"method_name"`
	Amount               string          `json:"amount"`
	Asset                string          `json:"asset,omitempty"`
	Network              string          `json:"network,omitempty"`
	Protocol             string          `json:"protocol,omitempty"`
	Integration          string          `json:"integration,omitempty"`
	OriginID             string          `json:"origin_id,omitempty"`
	Details              json.RawMessage `json:"details,omitempty"`
	LegalOperationEntity json.RawMessage `json:"legal_operation_entity,omitempty"`
}

type WithdrawalsResponse struct {
	apiEnvelope
	Payload []Withdrawal `json:"payload"`
}

// BookFee es la comisión vigente del usuario para un libro (/fees).
type BookFee struct {
	Book            string `json:"book"`
	FeeDecimal      string `json:"fee_decimal"`
	TakerFeeDecimal string `json:"taker_fee_decimal"`
	MakerFeeDecimal string `json:"maker_fee_decimal"`
}

type FeesResponse struct {
	apiEnvelope
	Payload struct {
		Fees []BookFee `json:"fees"`
	} `json:"payload"`
}
