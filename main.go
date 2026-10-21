package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"text/template"

	"github.com/gorilla/mux"
	"github.com/quickfixgo/traderui/basic"
	"github.com/quickfixgo/traderui/locate"
	"github.com/quickfixgo/traderui/oms"
	"github.com/quickfixgo/traderui/secmaster"

	"github.com/quickfixgo/quickfix"
	"github.com/quickfixgo/quickfix/store/file"
)

type fixFactory interface {
	NewOrderSingle(ord oms.Order) (msg quickfix.Messagable, err error)
	OrderCancelRequest(ord oms.Order, clOrdID string) (msg quickfix.Messagable, err error)
	SecurityDefinitionRequest(req secmaster.SecurityDefinitionRequest) (msg quickfix.Messagable, err error)
	QuoteRequest(req locate.QuoteRequest) (msg quickfix.Messagable, err error)
	OrderStatusRequest(req locate.OrderStatusRequest) (msg quickfix.Messagable, err error)
}

type tradeClient struct {
	SessionIDs map[string]quickfix.SessionID
	fixFactory
	*oms.OrderManager
}

func newTradeClient(factory fixFactory, idGen oms.ClOrdIDGenerator) *tradeClient {
	tc := &tradeClient{
		SessionIDs:   make(map[string]quickfix.SessionID),
		fixFactory:   factory,
		OrderManager: oms.NewOrderManager(idGen),
	}

	return tc
}

func (c tradeClient) SessionsAsJSON() (string, error) {
	sessionIDs := make([]string, 0, len(c.SessionIDs))

	for s := range c.SessionIDs {
		sessionIDs = append(sessionIDs, s)
	}

	b, err := json.Marshal(sessionIDs)
	return string(b), err
}

func (c tradeClient) OrdersAsJSON() (string, error) {
	c.RLock()
	defer c.RUnlock()

	b, err := json.Marshal(c.GetAll())
	return string(b), err
}

func (c tradeClient) ExecutionsAsJSON() (string, error) {
	c.RLock()
	defer c.RUnlock()

	b, err := json.Marshal(c.GetAllExecutions())
	return string(b), err
}

func (c tradeClient) MessagesAsJSON() (string, error) {
	c.RLock()
	defer c.RUnlock()

	b, err := json.Marshal(c.GetAllMessages())
	return string(b), err
}

func (c tradeClient) traderView(w http.ResponseWriter, r *http.Request) {
	var templates = template.Must(template.New("traderui").ParseFiles("tmpl/index.html"))
	if err := templates.ExecuteTemplate(w, "index.html", c); err != nil {
		log.Printf("[ERROR] err = %+v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (c tradeClient) fetchRequestedOrder(r *http.Request) (*oms.Order, error) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		panic(err)
	}

	return c.Get(id)
}

func (c tradeClient) fetchRequestedExecution(r *http.Request) (*oms.Execution, error) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		panic(err)
	}

	return c.GetExecution(id)
}

func (c tradeClient) getOrder(w http.ResponseWriter, r *http.Request) {
	c.RLock()
	defer c.RUnlock()

	order, err := c.fetchRequestedOrder(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	c.writeOrderJSON(w, order)
}

func (c tradeClient) writeOrderJSON(w http.ResponseWriter, order *oms.Order) {
	outgoingJSON, err := json.Marshal(order)
	if err != nil {
		log.Printf("[ERROR] err = %+v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, string(outgoingJSON))
}

func (c tradeClient) getExecution(w http.ResponseWriter, r *http.Request) {
	c.RLock()
	defer c.RUnlock()

	exec, err := c.fetchRequestedExecution(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	outgoingJSON, err := json.Marshal(exec)
	if err != nil {
		log.Printf("[ERROR] err = %+v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, string(outgoingJSON))
}

func (c tradeClient) deleteOrder(w http.ResponseWriter, r *http.Request) {
	c.Lock()
	defer c.Unlock()

	order, err := c.fetchRequestedOrder(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	clOrdID := c.AssignNextClOrdID(order)
	msg, err := c.OrderCancelRequest(*order, clOrdID)
	if err != nil {
		log.Printf("[ERROR] err = %+v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = quickfix.SendToTarget(msg, order.SessionID)
	if err != nil {
		log.Printf("[ERROR] err = %+v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	c.writeOrderJSON(w, order)
}

func (c tradeClient) getOrders(w http.ResponseWriter, r *http.Request) {
	outgoingJSON, err := c.OrdersAsJSON()
	if err != nil {
		log.Printf("[ERROR] err = %+v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, outgoingJSON)
}

func (c tradeClient) getExecutions(w http.ResponseWriter, r *http.Request) {
	outgoingJSON, err := c.ExecutionsAsJSON()
	if err != nil {
		log.Printf("[ERROR] err = %+v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, outgoingJSON)
}

func (c tradeClient) newSecurityDefintionRequest(w http.ResponseWriter, r *http.Request) {
	var secDefRequest secmaster.SecurityDefinitionRequest
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&secDefRequest)
	if err != nil {
		log.Printf("[ERROR] %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("secDefRequest = %+v\n", secDefRequest)

	if sessionID, ok := c.SessionIDs[secDefRequest.Session]; ok {
		secDefRequest.SessionID = sessionID
	} else {
		log.Println("[ERROR] Invalid SessionID")
		http.Error(w, "Invalid SessionID", http.StatusBadRequest)
		return
	}

	msg, err := c.fixFactory.SecurityDefinitionRequest(secDefRequest)
	if err != nil {
		log.Printf("[ERROR] %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = quickfix.SendToTarget(msg, secDefRequest.SessionID)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (c tradeClient) getMessages(w http.ResponseWriter, r *http.Request) {
	outgoingJSON, err := c.MessagesAsJSON()
	if err != nil {
		log.Printf("[ERROR] err = %+v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, outgoingJSON)
}

func (c tradeClient) newQuoteRequest(w http.ResponseWriter, r *http.Request) {
	var req locate.QuoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[ERROR] %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sessionID, ok := c.SessionIDs[req.Session]
	if !ok {
		log.Println("[ERROR] Invalid SessionID")
		http.Error(w, "Invalid SessionID", http.StatusBadRequest)
		return
	}
	req.SessionID = sessionID

	if req.Symbol == "" {
		http.Error(w, "symbol is required", http.StatusBadRequest)
		return
	}

	if req.QuoteReqID == "" {
		req.QuoteReqID = c.NextClOrdID()
	}

	msg, err := c.fixFactory.QuoteRequest(req)
	if err != nil {
		log.Printf("[ERROR] %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err = quickfix.SendToTarget(msg, req.SessionID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (c tradeClient) newOrderStatusRequest(w http.ResponseWriter, r *http.Request) {
	var req locate.OrderStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[ERROR] %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sessionID, ok := c.SessionIDs[req.Session]
	if !ok {
		log.Println("[ERROR] Invalid SessionID")
		http.Error(w, "Invalid SessionID", http.StatusBadRequest)
		return
	}
	req.SessionID = sessionID

	if req.OrderID == "" {
		http.Error(w, "order_id is required", http.StatusBadRequest)
		return
	}

	msg, err := c.fixFactory.OrderStatusRequest(req)
	if err != nil {
		log.Printf("[ERROR] %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err = quickfix.SendToTarget(msg, req.SessionID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (c tradeClient) newOrder(w http.ResponseWriter, r *http.Request) {
	var order oms.Order
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&order)
	if err != nil {
		log.Printf("[ERROR] %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if sessionID, ok := c.SessionIDs[order.Session]; ok {
		order.SessionID = sessionID
	} else {
		log.Println("[ERROR] Invalid SessionID")
		http.Error(w, "Invalid SessionID", http.StatusBadRequest)
		return
	}

	if err = order.Init(); err != nil {
		log.Printf("[ERROR] %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	c.Lock()
	_ = c.OrderManager.Save(&order)
	c.Unlock()

	msg, err := c.NewOrderSingle(order)
	if err != nil {
		log.Printf("[ERROR] %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = quickfix.SendToTarget(msg, order.SessionID)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func main() {
	fileStore := flag.Bool("file-store", false, "keep FIX sequence numbers and messages in FileStorePath, so sessions resume across restarts")
	clOrdIDPrefix := flag.String("clordid-prefix", "", "prefix for generated ClOrdIDs; set a unique value per run when the acceptor rejects duplicates across restarts")
	flag.Parse()

	cfgFileName := path.Join("config", "tradeclient.cfg")
	if flag.NArg() > 0 {
		cfgFileName = flag.Arg(0)
	}

	cfg, err := os.Open(cfgFileName)
	if err != nil {
		fmt.Printf("Error opening %v, %v\n", cfgFileName, err)
		return
	}

	appSettings, err := quickfix.ParseSettings(cfg)
	if err != nil {
		fmt.Println("Error reading cfg,", err)
		return
	}

	logFactory := NewFancyLog()

	var fixApp quickfix.Application
	app := newTradeClient(basic.FIXFactory{}, &basic.ClOrdIDGenerator{Prefix: *clOrdIDPrefix})
	fixApp = &basic.FIXApplication{
		SessionIDs:   app.SessionIDs,
		OrderManager: app.OrderManager,
	}

	storeFactory := quickfix.MessageStoreFactory(quickfix.NewMemoryStoreFactory())
	if *fileStore {
		storeFactory = file.NewStoreFactory(appSettings)
	}

	initiator, err := quickfix.NewInitiator(fixApp, storeFactory, appSettings, logFactory)
	if err != nil {
		log.Fatalf("Unable to create Initiator: %s\n", err)
	}

	if err = initiator.Start(); err != nil {
		log.Fatal(err)
	}
	defer initiator.Stop()

	router := mux.NewRouter().StrictSlash(true)

	router.HandleFunc("/orders", app.newOrder).Methods("POST")
	router.HandleFunc("/orders", app.getOrders).Methods("GET")
	router.HandleFunc("/orders/{id:[0-9]+}", app.getOrder).Methods("GET")
	router.HandleFunc("/orders/{id:[0-9]+}", app.deleteOrder).Methods("DELETE")

	router.HandleFunc("/executions", app.getExecutions).Methods("GET")
	router.HandleFunc("/executions/{id:[0-9]+}", app.getExecution).Methods("GET")

	router.HandleFunc("/securitydefinitionrequest", app.newSecurityDefintionRequest).Methods("POST")

	router.HandleFunc("/quoterequests", app.newQuoteRequest).Methods("POST")
	router.HandleFunc("/orderstatusrequests", app.newOrderStatusRequest).Methods("POST")
	router.HandleFunc("/messages", app.getMessages).Methods("GET")

	router.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", http.FileServer(http.Dir("assets"))))
	router.HandleFunc("/", app.traderView)

	log.Fatal(http.ListenAndServe(":8080", router))
}
