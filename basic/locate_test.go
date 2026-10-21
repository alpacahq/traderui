package basic

import (
	"strings"
	"testing"

	"github.com/quickfixgo/enum"
	"github.com/quickfixgo/quickfix"
	"github.com/shopspring/decimal"

	"github.com/quickfixgo/traderui/locate"
	"github.com/quickfixgo/traderui/oms"
)

func fix42Session() quickfix.SessionID {
	return quickfix.SessionID{BeginString: quickfix.BeginStringFIX42, SenderCompID: "TEST", TargetCompID: "APCA"}
}

func pipes(m quickfix.Messagable) string {
	return strings.ReplaceAll(m.ToMessage().String(), "\x01", "|")
}

func TestQuoteRequest(t *testing.T) {
	msg, err := FIXFactory{}.QuoteRequest(locate.QuoteRequest{
		SessionID:  fix42Session(),
		QuoteReqID: "q1",
		Symbol:     "GME",
	})
	if err != nil {
		t.Fatal(err)
	}

	got := pipes(msg)
	for _, want := range []string{"|35=R|", "|131=q1|", "|146=1|", "|55=GME|"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %s", want, got)
		}
	}
}

func TestOrderStatusRequestCarriesOnlyOrderID(t *testing.T) {
	msg, err := FIXFactory{}.OrderStatusRequest(locate.OrderStatusRequest{
		SessionID: fix42Session(),
		OrderID:   "0b9d2f6e",
	})
	if err != nil {
		t.Fatal(err)
	}

	got := pipes(msg)
	if !strings.Contains(got, "|35=H|") || !strings.Contains(got, "|37=0b9d2f6e|") {
		t.Errorf("unexpected message %s", got)
	}
	for _, unwanted := range []string{"|11=", "|55=", "|54="} {
		if strings.Contains(got, unwanted) {
			t.Errorf("found %q in %s", unwanted, got)
		}
	}
}

func TestLocateRequestsRejectNonFIX42(t *testing.T) {
	sid := quickfix.SessionID{BeginString: quickfix.BeginStringFIX44}

	if _, err := (FIXFactory{}).QuoteRequest(locate.QuoteRequest{SessionID: sid}); err == nil {
		t.Error("QuoteRequest: expected error for FIX.4.4")
	}
	if _, err := (FIXFactory{}).OrderStatusRequest(locate.OrderStatusRequest{SessionID: sid}); err == nil {
		t.Error("OrderStatusRequest: expected error for FIX.4.4")
	}
}

func TestNewOrderSingleTimeInForce(t *testing.T) {
	order := oms.Order{
		SessionID:       fix42Session(),
		ClOrdID:         "1",
		Symbol:          "GME",
		Side:            enum.Side_BUY,
		OrdType:         enum.OrdType_MARKET,
		QuantityDecimal: decimal.NewFromInt(100),
	}

	msg, err := FIXFactory{}.NewOrderSingle(order)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(pipes(msg), "|59=") {
		t.Errorf("TimeInForce sent although unset: %s", pipes(msg))
	}

	order.TimeInForce = enum.TimeInForce_FILL_OR_KILL
	msg, err = FIXFactory{}.NewOrderSingle(order)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(pipes(msg), "|59=4|") {
		t.Errorf("expected 59=4 in %s", pipes(msg))
	}
}

func TestFromAppRecordsInboundMessages(t *testing.T) {
	app := &FIXApplication{
		SessionIDs:   map[string]quickfix.SessionID{},
		OrderManager: oms.NewOrderManager(new(ClOrdIDGenerator)),
	}

	for _, msgType := range []string{"S", "b", "j"} {
		msg := quickfix.NewMessage()
		msg.Header.SetString(35, msgType)
		msg.Body.SetString(58, "text-"+msgType)

		if rej := app.FromApp(msg, fix42Session()); rej != nil {
			t.Fatalf("%s: unexpected reject: %v", msgType, rej)
		}
	}

	got := app.GetAllMessages()
	if len(got) != 3 {
		t.Fatalf("got %d messages, want 3", len(got))
	}
	for _, m := range got {
		if strings.Contains(m.Raw, "\x01") || !strings.Contains(m.Raw, "|58=text-"+m.MsgType+"|") {
			t.Errorf("unexpected Raw %q for %s", m.Raw, m.MsgType)
		}
	}
}
