package basic

import (
	"errors"
	"strconv"
	"time"

	"github.com/quickfixgo/enum"
	"github.com/quickfixgo/field"
	"github.com/quickfixgo/tag"
	"github.com/quickfixgo/traderui/oms"
	"github.com/quickfixgo/traderui/secmaster"
	"github.com/shopspring/decimal"

	fix40nos "github.com/quickfixgo/fix40/newordersingle"
	fix41nos "github.com/quickfixgo/fix41/newordersingle"
	fix42nos "github.com/quickfixgo/fix42/newordersingle"
	fix43nos "github.com/quickfixgo/fix43/newordersingle"
	fix44nos "github.com/quickfixgo/fix44/newordersingle"
	fix50nos "github.com/quickfixgo/fix50/newordersingle"

	fix42cxl "github.com/quickfixgo/fix42/ordercancelrequest"

	"github.com/quickfixgo/quickfix"
)

// FIXFactory builds vanilla fix messages, implements traderui.fixFactory
type FIXFactory struct{}

func (FIXFactory) NewOrderSingle(order oms.Order) (msg quickfix.Messagable, err error) {
	switch order.SessionID.BeginString {
	case quickfix.BeginStringFIX40:
		msg, err = nos40(order)
	case quickfix.BeginStringFIX41:
		msg, err = nos41(order)
	case quickfix.BeginStringFIX42:
		msg, err = nos42(order)
	case quickfix.BeginStringFIX43:
		msg, err = nos43(order)
	case quickfix.BeginStringFIX44:
		msg, err = nos44(order)
	case quickfix.BeginStringFIXT11:
		msg, err = nos50(order)
	default:
		err = errors.New("Unhandled BeginString")
	}

	return
}

func (FIXFactory) OrderCancelRequest(order oms.Order, clOrdID string) (msg quickfix.Messagable, err error) {
	switch order.SessionID.BeginString {
	case quickfix.BeginStringFIX42:
		msg, err = cxl42(order, clOrdID)
	default:
		err = errors.New("Unhandled BeginString")
	}

	return
}

// OrderCancelReplaceRequest builds a raw FIX G (OrderCancelReplaceRequest) message.
func (FIXFactory) OrderCancelReplaceRequest(order oms.Order, clOrdID string) (quickfix.Messagable, error) {
	m := quickfix.NewMessage()
	m.Header.SetField(tag.MsgType, quickfix.FIXString("G"))
	m.Body.SetField(tag.ClOrdID, quickfix.FIXString(clOrdID))
	m.Body.SetField(tag.OrigClOrdID, quickfix.FIXString(order.ClOrdID))
	m.Body.SetField(tag.HandlInst, quickfix.FIXString("1"))
	m.Body.SetField(tag.Symbol, quickfix.FIXString(order.Symbol))
	m.Body.SetField(tag.Side, quickfix.FIXString(string(order.Side)))
	m.Body.Set(field.NewTransactTime(time.Now()))
	m.Body.SetField(tag.OrdType, quickfix.FIXString(string(order.OrdType)))
	m.Body.SetField(tag.Account, quickfix.FIXString(order.Account))
	m.Body.Set(field.NewOrderQty(order.QuantityDecimal, 0))
	m.Body.SetField(tag.TimeInForce, quickfix.FIXString(string(order.Tif)))

	switch order.OrdType {
	case enum.OrdType_LIMIT, enum.OrdType_STOP_LIMIT:
		m.Body.Set(field.NewPrice(order.PriceDecimal, 2))
	}
	switch order.OrdType {
	case enum.OrdType_STOP, enum.OrdType_STOP_LIMIT:
		m.Body.Set(field.NewStopPx(order.StopPriceDecimal, 2))
	}

	return m, nil
}

// MultilegOrderCancelReplace builds a raw FIX AC (MultilegOrderCancelReplace) message.
func (FIXFactory) MultilegOrderCancelReplace(order oms.Order, clOrdID string) (quickfix.Messagable, error) {
	if len(order.Legs) == 0 {
		return nil, errors.New("multileg cancel/replace requires at least one leg")
	}

	m := quickfix.NewMessage()
	m.Header.SetField(tag.MsgType, quickfix.FIXString("AC"))
	m.Body.SetField(tag.ClOrdID, quickfix.FIXString(clOrdID))
	m.Body.SetField(tag.OrigClOrdID, quickfix.FIXString(order.ClOrdID))
	m.Body.SetField(tag.HandlInst, quickfix.FIXString("1"))
	m.Body.SetField(tag.Symbol, quickfix.FIXString(order.Symbol))
	m.Body.Set(field.NewTransactTime(time.Now()))
	m.Body.SetField(tag.OrdType, quickfix.FIXString(string(order.OrdType)))
	m.Body.SetField(tag.Account, quickfix.FIXString(order.Account))
	m.Body.Set(field.NewOrderQty(order.QuantityDecimal, 0))
	m.Body.SetField(tag.SecurityType, quickfix.FIXString("MLEG"))

	switch order.OrdType {
	case enum.OrdType_LIMIT, enum.OrdType_STOP_LIMIT:
		m.Body.Set(field.NewPrice(order.PriceDecimal, 2))
	}

	m.Body.SetGroup(buildLegsGroup(order.Legs))
	return m, nil
}


func (FIXFactory) SecurityDefinitionRequest(req secmaster.SecurityDefinitionRequest) (msg quickfix.Messagable, err error) {
	err = errors.New("Not Implemented")
	return
}

// NewOrderMultileg builds a raw FIX AB (NewOrderMultileg) message.
// Sent on a FIX 4.2 session using FIX 4.4 multileg tags.
func (FIXFactory) NewOrderMultileg(order oms.Order) (quickfix.Messagable, error) {
	if len(order.Legs) == 0 {
		return nil, errors.New("multileg order requires at least one leg")
	}

	m := quickfix.NewMessage()
	m.Header.SetField(tag.MsgType, quickfix.FIXString("AB"))
	m.Body.SetField(tag.ClOrdID, quickfix.FIXString(order.ClOrdID))
	m.Body.SetField(tag.HandlInst, quickfix.FIXString("1"))
	m.Body.SetField(tag.Symbol, quickfix.FIXString(order.Symbol))
	m.Body.Set(field.NewTransactTime(time.Now()))
	m.Body.SetField(tag.OrdType, quickfix.FIXString(string(order.OrdType)))
	m.Body.SetField(tag.Account, quickfix.FIXString(order.Account))
	m.Body.Set(field.NewOrderQty(order.QuantityDecimal, 0))
	m.Body.SetField(tag.TimeInForce, quickfix.FIXString(string(order.Tif)))
	m.Body.SetField(tag.SecurityType, quickfix.FIXString("MLEG"))

	switch order.OrdType {
	case enum.OrdType_LIMIT, enum.OrdType_STOP_LIMIT:
		m.Body.Set(field.NewPrice(order.PriceDecimal, 2))
	}

	m.Body.SetGroup(buildLegsGroup(order.Legs))
	return m, nil
}

func buildLegsGroup(legs []oms.Leg) *quickfix.RepeatingGroup {
	group := quickfix.NewRepeatingGroup(
		tag.NoLegs,
		quickfix.GroupTemplate{
			quickfix.GroupElement(tag.LegSymbol),
			quickfix.GroupElement(tag.LegRefID),
			quickfix.GroupElement(tag.LegCFICode),
			quickfix.GroupElement(tag.LegStrikePrice),
			quickfix.GroupElement(tag.LegMaturityDate),
			quickfix.GroupElement(tag.LegSide),
			quickfix.GroupElement(tag.LegRatioQty),
			quickfix.GroupElement(tag.LegPositionEffect),
		},
	)

	for i, leg := range legs {
		g := group.Add()
		g.SetField(tag.LegSymbol, quickfix.FIXString(leg.Symbol))
		g.SetField(tag.LegRefID, quickfix.FIXString(strconv.Itoa(i)))
		g.SetField(tag.LegCFICode, quickfix.FIXString(leg.CFICode))
		if leg.IsOption() {
			strikePrice, _ := decimal.NewFromString(leg.StrikePrice)
			g.SetField(tag.LegStrikePrice, quickfix.FIXDecimal{Decimal: strikePrice, Scale: 0})
			g.SetField(tag.LegMaturityDate, quickfix.FIXString(leg.MaturityDate))
		}
		g.SetField(tag.LegSide, quickfix.FIXString(leg.Side))
		ratioQty := decimal.NewFromInt(int64(leg.RatioQty))
		g.SetField(tag.LegRatioQty, quickfix.FIXDecimal{Decimal: ratioQty, Scale: 0})
		g.SetField(tag.LegPositionEffect, quickfix.FIXString(leg.PositionEffect))
	}

	return group
}

func populateOrder(genMessage quickfix.Messagable, ord oms.Order) (quickfix.Messagable, error) {
	msg := genMessage.ToMessage()

	switch ord.OrdType {
	case enum.OrdType_LIMIT, enum.OrdType_STOP_LIMIT:
		msg.Body.Set(field.NewPrice(ord.PriceDecimal, 2))
	}

	switch ord.OrdType {
	case enum.OrdType_STOP, enum.OrdType_STOP_LIMIT:
		msg.Body.Set(field.NewStopPx(ord.StopPriceDecimal, 2))
	}

	if len(ord.SecurityType) > 0 {
		msg.Body.Set(field.NewSecurityType(ord.SecurityType))
	}

	if len(ord.MaturityMonthYear) > 0 {
		msg.Body.Set(field.NewMaturityMonthYear(ord.MaturityMonthYear))
	}

	if ord.MaturityDay > 0 {
		msg.Body.SetInt(tag.MaturityDay, ord.MaturityDay)
	}

	if ord.SecurityType == enum.SecurityType_OPTION {
		msg.Body.Set(field.NewPutOrCall(ord.PutOrCall))
		if !ord.StrikePriceDecimal.IsZero() {
			msg.Body.Set(field.NewStrikePrice(ord.StrikePriceDecimal, 2))
		}
	}

	return msg, nil
}

func nos40(ord oms.Order) (quickfix.Messagable, error) {
	nos := fix40nos.New(
		field.NewClOrdID(ord.ClOrdID),
		field.NewHandlInst("1"),
		field.NewSymbol(ord.Symbol),
		field.NewSide(ord.Side),
		field.NewOrderQty(ord.QuantityDecimal, 0),
		field.NewOrdType(ord.OrdType),
	)

	return populateOrder(nos, ord)
}

func nos41(ord oms.Order) (quickfix.Messagable, error) {
	nos := fix41nos.New(
		field.NewClOrdID(ord.ClOrdID),
		field.NewHandlInst("1"),
		field.NewSymbol(ord.Symbol),
		field.NewSide(ord.Side),
		field.NewOrdType(ord.OrdType),
	)
	nos.Set(field.NewOrderQty(ord.QuantityDecimal, 0))

	return populateOrder(nos, ord)
}

func nos42(ord oms.Order) (quickfix.Messagable, error) {
	nos := fix42nos.New(
		field.NewClOrdID(ord.ClOrdID),
		field.NewHandlInst("1"),
		field.NewSymbol(ord.Symbol),
		field.NewSide(ord.Side),
		field.NewTransactTime(time.Now()),
		field.NewOrdType(ord.OrdType),
	)
	nos.Set(field.NewOrderQty(ord.QuantityDecimal, 0))
	nos.Set(field.NewAccount(ord.Account))
	nos.Set(field.NewTimeInForce(ord.Tif))
	if len(ord.OpenClose) > 0 {
		nos.Set(field.NewOpenClose(ord.OpenClose))
	}

	return populateOrder(nos, ord)
}

func cxl42(ord oms.Order, clOrdID string) (quickfix.Messagable, error) {
	cxl := fix42cxl.New(
		field.NewOrigClOrdID(ord.ClOrdID),
		field.NewClOrdID(clOrdID),
		field.NewSymbol(ord.Symbol),
		field.NewSide(ord.Side),
		field.NewTransactTime(time.Now()),
	)
	cxl.Set(field.NewAccount(ord.Account))

	return cxl, nil
}

func nos43(ord oms.Order) (quickfix.Messagable, error) {
	nos := fix43nos.New(
		field.NewClOrdID(ord.ClOrdID),
		field.NewHandlInst("1"),
		field.NewSide(ord.Side),
		field.NewTransactTime(time.Now()),
		field.NewOrdType(ord.OrdType),
	)
	nos.Set(field.NewSymbol(ord.Symbol))
	nos.Set(field.NewOrderQty(ord.QuantityDecimal, 0))

	return populateOrder(nos, ord)
}

func nos44(ord oms.Order) (quickfix.Messagable, error) {
	nos := fix44nos.New(
		field.NewClOrdID(ord.ClOrdID),
		field.NewSide(ord.Side),
		field.NewTransactTime(time.Now()),
		field.NewOrdType(ord.OrdType),
	)
	nos.Set(field.NewSymbol(ord.Symbol))
	nos.Set(field.NewHandlInst("1"))
	nos.Set(field.NewOrderQty(ord.QuantityDecimal, 0))

	return populateOrder(nos, ord)
}

func nos50(ord oms.Order) (quickfix.Messagable, error) {
	nos := fix50nos.New(
		field.NewClOrdID(ord.ClOrdID),
		field.NewSide(ord.Side),
		field.NewTransactTime(time.Now()),
		field.NewOrdType(ord.OrdType),
	)
	nos.Set(field.NewHandlInst("1"))
	nos.Set(field.NewSymbol(ord.Symbol))
	nos.Set(field.NewOrderQty(ord.QuantityDecimal, 0))

	return populateOrder(nos, ord)
}
