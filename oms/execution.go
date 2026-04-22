package oms

import "github.com/quickfixgo/enum"

// Execution is the execution type
type Execution struct {
	ID                int                `json:"id"`
	Symbol            string             `json:"symbol"`
	Quantity          string             `json:"quantity"`
	Side              enum.Side          `json:"side"`
	Price             string             `json:"price"`
	Session           string             `json:"session_id"`
	SecurityType      enum.SecurityType  `json:"security_type"`
	MaturityMonthYear string             `json:"maturity_month_year"`
	PutOrCall         enum.PutOrCall     `json:"put_or_call"`
	StrikePrice       string             `json:"strike_price"`
	IsLegExecution    bool               `json:"is_leg"`
	ClOrdID           string             `json:"clord_id"`
	OrderID           string             `json:"order_id"`
	ExecID            string             `json:"exec_id"`
	OrdStatus         string             `json:"ord_status"`
}
