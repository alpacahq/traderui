package oms

import (
	"errors"

	"github.com/quickfixgo/enum"
	"github.com/shopspring/decimal"

	"github.com/quickfixgo/quickfix"
)

// Leg represents a single leg in a multileg order
type Leg struct {
	Symbol         string `json:"leg_symbol"`
	CFICode        string `json:"leg_cfi_code"`         // OC=Call, OP=Put, ES=Equity
	StrikePrice    string `json:"leg_strike_price"`
	MaturityDate   string `json:"leg_maturity_date"`    // YYYYMMDD
	Side           string `json:"leg_side"`             // 1=Buy, 2=Sell
	RatioQty       int    `json:"leg_ratio_qty"`
	PositionEffect string `json:"leg_position_effect"`  // O=Open, C=Close
}

// IsOption returns true if the leg represents a call or put
func (l Leg) IsOption() bool {
	return l.CFICode == "OC" || l.CFICode == "OP"
}

// Order is the order type
type Order struct {
	ID                 int                `json:"id"`
	SessionID          quickfix.SessionID `json:"-"`
	ClOrdID            string             `json:"clord_id"`
	OrderID            string             `json:"order_id"`
	Symbol             string             `json:"symbol"`
	QuantityDecimal    decimal.Decimal    `json:"-"`
	Quantity           string             `json:"quantity"`
	Account            string             `json:"account"`
	Session            string             `json:"session_id"`
	Side               enum.Side          `json:"side"`
	OrdType            enum.OrdType       `json:"ord_type"`
	PriceDecimal       decimal.Decimal    `json:"-"`
	Price              string             `json:"price"`
	StopPriceDecimal   decimal.Decimal    `json:"-"`
	StopPrice          string             `json:"stop_price"`
	Closed             string             `json:"closed"`
	Open               string             `json:"open"`
	AvgPx              string             `json:"avg_px"`
	SecurityType       enum.SecurityType  `json:"security_type"`
	SecurityDesc       string             `json:"security_desc"`
	MaturityMonthYear  string             `json:"maturity_month_year"`
	MaturityDay        int                `json:"maturity_day"`
	PutOrCall          enum.PutOrCall     `json:"put_or_call"`
	StrikePrice        string             `json:"strike_price"`
	StrikePriceDecimal decimal.Decimal    `json:"-"`
	Tif                enum.TimeInForce   `json:"tif"`
	OpenClose          enum.OpenClose     `json:"open_close"`
	Legs               []Leg              `json:"legs,omitempty"`
	OrdStatus          string             `json:"ord_status"`
	ExecType           string             `json:"exec_type"`
	RejectionReason    string             `json:"rejection_reason,omitempty"`
}

// Init initialized computed fields on order from user input
func (order *Order) Init() error {
	var err error
	if order.QuantityDecimal, err = decimal.NewFromString(order.Quantity); err != nil {
		return errors.New("Invalid Qty")
	}

	if order.StrikePrice != "" {
		if order.StrikePriceDecimal, err = decimal.NewFromString(order.StrikePrice); err != nil {
			return errors.New("Invalid StrikePrice")
		}
	}

	switch order.OrdType {
	case enum.OrdType_LIMIT, enum.OrdType_STOP_LIMIT:
		if order.PriceDecimal, err = decimal.NewFromString(order.Price); err != nil {
			return errors.New("Invalid Price")
		}
	}

	switch order.OrdType {
	case enum.OrdType_STOP, enum.OrdType_STOP_LIMIT:
		if order.StopPriceDecimal, err = decimal.NewFromString(order.StopPrice); err != nil {
			return errors.New("Invalid StopPrice")
		}
	}

	return nil
}
