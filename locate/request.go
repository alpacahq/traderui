// Package locate holds the request types for the Locate FIX profile
// (QuoteRequest and OrderStatusRequest).
package locate

import "github.com/quickfixgo/quickfix"

// QuoteRequest asks for an availability snapshot for one symbol.
type QuoteRequest struct {
	SessionID  quickfix.SessionID `json:"-"`
	Session    string             `json:"session_id"`
	QuoteReqID string             `json:"quote_req_id"`
	Symbol     string             `json:"symbol"`
}

// OrderStatusRequest looks up a Locate by the OrderID returned when it was created.
type OrderStatusRequest struct {
	SessionID quickfix.SessionID `json:"-"`
	Session   string             `json:"session_id"`
	OrderID   string             `json:"order_id"`
}
