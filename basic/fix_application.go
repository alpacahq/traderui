package basic

import (
	"log"
	"strconv"
	"sync"

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

	statusMu sync.RWMutex
	loggedOn map[string]bool

	// seqMu guards clOrdBySeq. It's populated in ToApp for outgoing
	// order-related messages so that a BusinessMessageReject carrying
	// only RefSeqNum (tag 45) can still be mapped back to its ClOrdID.
	seqMu       sync.RWMutex
	clOrdBySeq  map[string]string // key: sessionID + "/" + seqNum
}

// SessionStatus reports whether each known session is currently logged on.
func (a *FIXApplication) SessionStatus() map[string]bool {
	a.statusMu.RLock()
	defer a.statusMu.RUnlock()
	out := make(map[string]bool, len(a.SessionIDs))
	for id := range a.SessionIDs {
		out[id] = a.loggedOn[id]
	}
	return out
}

func (a *FIXApplication) setLoggedOn(sessionID quickfix.SessionID, on bool) {
	a.statusMu.Lock()
	if a.loggedOn == nil {
		a.loggedOn = make(map[string]bool)
	}
	a.loggedOn[sessionID.String()] = on
	a.statusMu.Unlock()
}

// OnLogon records the session as logged on.
func (a *FIXApplication) OnLogon(sessionID quickfix.SessionID) {
	a.setLoggedOn(sessionID, true)
	log.Printf("[INFO] session logon: %s", sessionID)
}

// OnLogout records the session as logged out.
func (a *FIXApplication) OnLogout(sessionID quickfix.SessionID) {
	a.setLoggedOn(sessionID, false)
	log.Printf("[INFO] session logout: %s", sessionID)
}

// ToAdmin is ignored
func (a *FIXApplication) ToAdmin(msg *quickfix.Message, sessionID quickfix.SessionID) {}

// OnCreate initialized SessionIDs
func (a *FIXApplication) OnCreate(sessionID quickfix.SessionID) {
	a.SessionIDs[sessionID.String()] = sessionID
	a.setLoggedOn(sessionID, false)
}

// FromAdmin is ignored
func (a *FIXApplication) FromAdmin(msg *quickfix.Message, sessionID quickfix.SessionID) (reject quickfix.MessageRejectError) {
	return
}

// ToApp captures outgoing order-related messages so we can look up a
// ClOrdID from a RefSeqNum when a Business Message Reject does not include
// BusinessRejectRefID (tag 379).
func (a *FIXApplication) ToApp(msg *quickfix.Message, sessionID quickfix.SessionID) (err error) {
	msgType, mtErr := msg.MsgType()
	if mtErr != nil {
		return
	}

	switch msgType {
	case "D", "AB", "F", "G", "AC":
	default:
		return
	}

	var seqNum quickfix.FIXInt
	if hErr := msg.Header.GetField(tag.MsgSeqNum, &seqNum); hErr != nil {
		return
	}

	clOrdID := getStringTag(msg, tag.ClOrdID, "")
	if clOrdID == "" {
		return
	}

	a.seqMu.Lock()
	if a.clOrdBySeq == nil {
		a.clOrdBySeq = make(map[string]string)
	}
	a.clOrdBySeq[seqKey(sessionID, int(seqNum))] = clOrdID
	a.seqMu.Unlock()
	return
}

func seqKey(sessionID quickfix.SessionID, seqNum int) string {
	return sessionID.String() + "/" + strconv.Itoa(seqNum)
}

func (a *FIXApplication) lookupClOrdIDBySeq(sessionID quickfix.SessionID, seqNum int) (string, bool) {
	a.seqMu.RLock()
	defer a.seqMu.RUnlock()
	id, ok := a.clOrdBySeq[seqKey(sessionID, seqNum)]
	return id, ok
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
	case enum.MsgType_ORDER_CANCEL_REJECT:
		return a.onOrderCancelReject(msg, sessionID)
	case enum.MsgType_BUSINESS_MESSAGE_REJECT:
		return a.onBusinessMessageReject(msg, sessionID)
	}

	return quickfix.UnsupportedMessageType()
}

func (a *FIXApplication) onOrderCancelReject(msg *quickfix.Message, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	a.Lock()
	defer a.Unlock()

	text := getStringTag(msg, tag.Text, "")
	rejRespTo := getStringTag(msg, tag.CxlRejResponseTo, "")
	log.Printf("[WARN] Order Cancel Reject: respTo=%s text=%s", rejRespTo, text)

	lookup := getStringTag(msg, tag.OrigClOrdID, "")
	if lookup == "" {
		lookup = getStringTag(msg, tag.ClOrdID, "")
	}
	if lookup == "" {
		return nil
	}

	order, err := a.GetByClOrdID(lookup)
	if err != nil {
		return nil
	}

	// OrdStatus on an OrderCancelReject reflects the CURRENT state of the
	// order as known by the broker (typically still NEW/PARTIAL_FILL). It
	// is not a terminal rejection of the order itself.
	if status := getStringTag(msg, tag.OrdStatus, ""); status != "" {
		order.OrdStatus = status
	}

	reason := text
	if reason == "" {
		reason = "Cancel/Replace Rejected"
	}
	order.LastOpRejection = reason

	return nil
}

func (a *FIXApplication) onBusinessMessageReject(msg *quickfix.Message, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	a.Lock()
	defer a.Unlock()

	text := getStringTag(msg, tag.Text, "")
	refMsgType := getStringTag(msg, tag.RefMsgType, "")
	refID := getStringTag(msg, tag.BusinessRejectRefID, "")
	rejCode := getStringTag(msg, tag.BusinessRejectReason, "")
	refSeqNum := getStringTag(msg, tag.RefSeqNum, "")

	log.Printf("[WARN] Business Message Reject: refMsgType=%s refID=%s refSeqNum=%s code=%s text=%s",
		refMsgType, refID, refSeqNum, rejCode, text)

	// Some counterparties don't echo BusinessRejectRefID; fall back to
	// RefSeqNum (tag 45) mapped via the outgoing seq -> ClOrdID table.
	if refID == "" && refSeqNum != "" {
		if seqNum, convErr := strconv.Atoi(refSeqNum); convErr == nil {
			if id, ok := a.lookupClOrdIDBySeq(sessionID, seqNum); ok {
				refID = id
				log.Printf("[INFO] Business Message Reject: resolved refSeqNum=%d -> clOrdID=%s", seqNum, refID)
			}
		}
	}

	if refID == "" {
		return nil
	}

	order, err := a.GetByClOrdID(refID)
	if err != nil {
		return nil
	}

	reason := text
	if reason == "" && rejCode != "" {
		reason = "Business reject code: " + rejCode
	} else if reason != "" && rejCode != "" {
		reason = reason + " (code: " + rejCode + ")"
	}
	if reason == "" {
		reason = "Business Message Reject"
	}

	// Cancel (F), CancelReplace (G), MultilegCancelReplace (AC) and
	// MultilegCancel (same F in FIX 4.2) all operate on an existing order;
	// a reject means the order is still live with its previous state.
	switch refMsgType {
	case "F", "G", "AC":
		order.LastOpRejection = reason
		return nil
	}

	// Reject is against a new-order message (D or AB) — the order itself
	// never made it. Mark it rejected.
	order.OrdStatus = string(enum.OrdStatus_REJECTED)
	order.ExecType = string(enum.ExecType_REJECTED)
	order.RejectionReason = reason
	order.Open = "0"

	return nil
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
		// Keep order.ClOrdID synced with the latest accepted ClOrdID so
		// that future amends/cancels use the correct OrigClOrdID.
		order.ClOrdID = clOrdID.String()

		if msg.Body.Has(tag.OrdStatus) {
			order.OrdStatus = getStringTag(msg, tag.OrdStatus, order.OrdStatus)
		}
		if msg.Body.Has(tag.ExecType) {
			order.ExecType = getStringTag(msg, tag.ExecType, order.ExecType)
		}
		if msg.Body.Has(tag.OrderID) {
			order.OrderID = getStringTag(msg, tag.OrderID, order.OrderID)
		}

		if order.OrdStatus == string(enum.OrdStatus_REJECTED) {
			if txt := getStringTag(msg, tag.Text, ""); txt != "" {
				order.RejectionReason = txt
			}
			if msg.Body.Has(tag.OrdRejReason) {
				rejCode := getStringTag(msg, tag.OrdRejReason, "")
				if rejCode != "" && order.RejectionReason != "" {
					order.RejectionReason = order.RejectionReason + " (code: " + rejCode + ")"
				} else if rejCode != "" {
					order.RejectionReason = "Rejection code: " + rejCode
				}
			}
			log.Printf("[WARN] Order REJECTED clOrdID=%s reason=%s", clOrdID.String(), order.RejectionReason)
		}

		if order.OrdStatus == string(enum.OrdStatus_CANCELED) {
			reason := getStringTag(msg, tag.Text, "")
			if reason != "" {
				order.RejectionReason = reason
			}
			log.Printf("[INFO] Order CANCELED clOrdID=%s reason=%s", clOrdID.String(), reason)
		}

		switch order.ExecType {
		case string(enum.ExecType_NEW),
			string(enum.ExecType_REPLACED),
			string(enum.ExecType_PARTIAL_FILL),
			string(enum.ExecType_FILL),
			string(enum.ExecType_TRADE):
			order.RejectionReason = ""
			order.LastOpRejection = ""
		}
	}

	if !isLegER {
		order.Closed = cumQty.String()
		order.Open = leavesQty.String()
		order.AvgPx = avgPx.String()

		if msg.Body.Has(tag.OrderQty) {
			order.Quantity = getStringTag(msg, tag.OrderQty, order.Quantity)
		}
		if msg.Body.Has(tag.Price) {
			order.Price = getStringTag(msg, tag.Price, order.Price)
		}
		if msg.Body.Has(tag.StopPx) {
			order.StopPrice = getStringTag(msg, tag.StopPx, order.StopPrice)
		}
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
		exec.ClOrdID = clOrdID.String()
		exec.IsLegExecution = isLegER
		exec.OrderID = getStringTag(msg, tag.OrderID, order.OrderID)
		exec.ExecID = getStringTag(msg, tag.ExecID, "")
		exec.OrdStatus = getStringTag(msg, tag.OrdStatus, "")

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
