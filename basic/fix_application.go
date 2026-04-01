package basic

import (
	"log"

	"github.com/quickfixgo/enum"
	"github.com/quickfixgo/field"
	"github.com/quickfixgo/tag"
	"github.com/quickfixgo/traderui/oms"

	"github.com/quickfixgo/quickfix"
)

// FIXApplication implements a basic quickfix.Application
type FIXApplication struct {
	SessionIDs map[string]quickfix.SessionID
	*oms.OrderManager
}

// OnLogon is ignored
func (a *FIXApplication) OnLogon(sessionID quickfix.SessionID) {}

// OnLogout is ignored
func (a *FIXApplication) OnLogout(sessionID quickfix.SessionID) {}

// ToAdmin is ignored
func (a *FIXApplication) ToAdmin(msg *quickfix.Message, sessionID quickfix.SessionID) {}

// OnCreate initialized SessionIDs
func (a *FIXApplication) OnCreate(sessionID quickfix.SessionID) {
	a.SessionIDs[sessionID.String()] = sessionID
}

// FromAdmin is ignored
func (a *FIXApplication) FromAdmin(msg *quickfix.Message, sessionID quickfix.SessionID) (reject quickfix.MessageRejectError) {
	return
}

// ToApp is ignored
func (a *FIXApplication) ToApp(msg *quickfix.Message, sessionID quickfix.SessionID) (err error) {
	return
}

// FromApp listens for just execution reports
func (a *FIXApplication) FromApp(msg *quickfix.Message, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	msgType, err := msg.MsgType()
	if err != nil {
		return err
	}

	switch enum.MsgType(msgType) {
	case enum.MsgType_EXECUTION_REPORT:
		return a.onExecutionReport(msg, sessionID)
	case enum.MsgType_BUSINESS_MESSAGE_REJECT:
		var text quickfix.FIXString
		if msg.Body.Has(tag.Text) {
			_ = msg.Body.GetField(tag.Text, &text)
		}
		log.Printf("[WARN] Business Message Reject: %s", string(text))
		return nil
	}

	return quickfix.UnsupportedMessageType()
}

func (a *FIXApplication) onExecutionReport(msg *quickfix.Message, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	a.Lock()
	defer a.Unlock()

	var clOrdID field.ClOrdIDField
	if err := msg.Body.Get(&clOrdID); err != nil {
		return err
	}

	order, err := a.GetByClOrdID(clOrdID.String())
	if err != nil {
		log.Printf("[ERROR] err= %v", err)
		return nil
	}

	var cumQty field.CumQtyField
	if err := msg.Body.Get(&cumQty); err != nil {
		return err
	}

	var avgPx field.AvgPxField
	if err := msg.Body.Get(&avgPx); err != nil {
		return err
	}

	var leavesQty field.LeavesQtyField
	if err := msg.Body.Get(&leavesQty); err != nil {
		return err
	}

	isLegER := false
	if msg.Body.Has(tag.MultiLegReportingType) {
		var mlrt field.MultiLegReportingTypeField
		if err := msg.Body.Get(&mlrt); err != nil {
			return err
		}
		switch mlrt.Value() {
		case enum.MultiLegReportingType_INDIVIDUAL_LEG_OF_A_MULTI_LEG_SECURITY:
			isLegER = true
		}
	}

	if !isLegER {
		order.Closed = cumQty.String()
		order.Open = leavesQty.String()
		order.AvgPx = avgPx.String()
	}

	if msg.Body.Has(tag.LastShares) {
		var lastShares field.LastSharesField
		if err := msg.Body.Get(&lastShares); err != nil {
			return err
		}

		if lastShares.Decimal.IsZero() {
			return nil
		}

		var price field.LastPxField
		if err := msg.Body.Get(&price); err != nil {
			return err
		}

		exec := new(oms.Execution)
		exec.Quantity = lastShares.String()
		exec.Price = price.String()
		exec.Session = order.Session

		if isLegER {
			exec.Symbol = getStringTag(msg, tag.Symbol, order.Symbol)
			exec.Side = enum.Side(getStringTag(msg, tag.Side, string(order.Side)))
			exec.SecurityType = enum.SecurityType(getStringTag(msg, tag.SecurityType, string(order.SecurityType)))

			if exec.SecurityType == enum.SecurityType_OPTION {
				exec.MaturityMonthYear = getStringTag(msg, tag.MaturityMonthYear, "")
				exec.StrikePrice = getStringTag(msg, tag.StrikePrice, "")
				if msg.Body.Has(tag.PutOrCall) {
					var poc field.PutOrCallField
					if err := msg.Body.Get(&poc); err == nil {
						exec.PutOrCall = poc.Value()
					}
				}
			}
		} else {
			exec.Symbol = order.Symbol
			exec.Side = order.Side
			exec.SecurityType = order.SecurityType
			exec.MaturityMonthYear = order.MaturityMonthYear
			exec.PutOrCall = order.PutOrCall
			exec.StrikePrice = order.StrikePrice
		}

		_ = a.SaveExecution(exec)
	}

	return nil
}

func getStringTag(msg *quickfix.Message, t quickfix.Tag, fallback string) string {
	var val quickfix.FIXString
	if err := msg.Body.GetField(t, &val); err == nil {
		return string(val)
	}
	return fallback
}
