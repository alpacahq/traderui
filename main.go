package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"text/template"

	"github.com/gorilla/mux"
	"github.com/quickfixgo/traderui/alpaca"
	"github.com/quickfixgo/traderui/basic"
	"github.com/quickfixgo/traderui/oms"
	"github.com/quickfixgo/traderui/secmaster"

	"github.com/quickfixgo/quickfix"
)

type SymbolEntry struct {
	Symbol            string `json:"symbol"`
	Type              string `json:"type"`
	Description       string `json:"description"`
	CFICode           string `json:"cfi_code,omitempty"`
	StrikePrice       string `json:"strike_price,omitempty"`
	MaturityDate      string `json:"maturity_date,omitempty"`
	MaturityMonthYear string `json:"maturity_month_year,omitempty"`
}

type SymbolsConfig struct {
	Symbols []SymbolEntry `json:"symbols"`
}

func loadSymbolsConfig(filePath string) (*SymbolsConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &SymbolsConfig{}, nil
		}
		return nil, fmt.Errorf("read %s: %w", filePath, err)
	}
	var cfg SymbolsConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("error parsing symbols config: %w", err)
	}
	return &cfg, nil
}

type fixFactory interface {
	NewOrderSingle(ord oms.Order) (msg quickfix.Messagable, err error)
	OrderCancelRequest(ord oms.Order, clOrdID string) (msg quickfix.Messagable, err error)
	OrderCancelReplaceRequest(ord oms.Order, clOrdID string) (msg quickfix.Messagable, err error)
	SecurityDefinitionRequest(req secmaster.SecurityDefinitionRequest) (msg quickfix.Messagable, err error)
	NewOrderMultileg(ord oms.Order) (msg quickfix.Messagable, err error)
	MultilegOrderCancelReplace(ord oms.Order, clOrdID string) (msg quickfix.Messagable, err error)
}

type sessionStatusSource interface {
	SessionStatus() map[string]bool
}

type tradeClient struct {
	SessionIDs    map[string]quickfix.SessionID
	symbolsConfig *SymbolsConfig
	statusSrc     sessionStatusSource
	fixFactory
	*oms.OrderManager
}

func newTradeClient(factory fixFactory, idGen oms.ClOrdIDGenerator, symbols *SymbolsConfig) *tradeClient {
	tc := &tradeClient{
		SessionIDs:    make(map[string]quickfix.SessionID),
		symbolsConfig: symbols,
		fixFactory:    factory,
		OrderManager:  oms.NewOrderManager(idGen),
	}

	return tc
}

func (c tradeClient) SymbolsAsJSON() (string, error) {
	if c.symbolsConfig == nil {
		return "[]", nil
	}
	b, err := json.Marshal(c.symbolsConfig.Symbols)
	return string(b), err
}

func (c tradeClient) SessionsAsJSON() (string, error) {
	sessionIDs := make([]string, 0, len(c.SessionIDs))

	for s := range c.SessionIDs {
		sessionIDs = append(sessionIDs, s)
	}

	b, err := json.Marshal(sessionIDs)
	return string(b), err
}

func (c tradeClient) getSessionStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]bool{}
	if c.statusSrc != nil {
		status = c.statusSrc.SessionStatus()
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(status)
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
		return nil, fmt.Errorf("invalid order id %q: %w", vars["id"], err)
	}

	return c.Get(id)
}

func (c tradeClient) fetchRequestedExecution(r *http.Request) (*oms.Execution, error) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		return nil, fmt.Errorf("invalid execution id %q: %w", vars["id"], err)
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

	if err = quickfix.SendToTarget(msg, order.SessionID); err != nil {
		log.Printf("[ERROR] %v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	c.writeOrderJSON(w, &order)
}

func (c tradeClient) updateOrder(w http.ResponseWriter, r *http.Request) {
	c.Lock()
	defer c.Unlock()

	order, err := c.fetchRequestedOrder(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	var update oms.Order
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&update); err != nil {
		log.Printf("[ERROR] %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := update.Init(); err != nil {
		log.Printf("[ERROR] %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pending := *order
	if update.Quantity != "" {
		pending.Quantity = update.Quantity
		pending.QuantityDecimal = update.QuantityDecimal
	}
	if update.OrdType != "" {
		pending.OrdType = update.OrdType
	}
	if update.Price != "" {
		pending.Price = update.Price
		pending.PriceDecimal = update.PriceDecimal
	}
	if update.StopPrice != "" {
		pending.StopPrice = update.StopPrice
		pending.StopPriceDecimal = update.StopPriceDecimal
	}

	clOrdID := c.AssignNextClOrdID(order)

	msg, err := c.OrderCancelReplaceRequest(pending, clOrdID)
	if err != nil {
		log.Printf("[ERROR] %v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = quickfix.SendToTarget(msg, order.SessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	order.ClOrdID = clOrdID
	c.writeOrderJSON(w, order)
}

var port = flag.String("port", "8080", "HTTP listen port")

func (c tradeClient) newMultilegOrder(w http.ResponseWriter, r *http.Request) {
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

	order.SecurityType = "MLEG"

	if err = order.Init(); err != nil {
		log.Printf("[ERROR] %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(order.Legs) == 0 {
		http.Error(w, "At least one leg is required", http.StatusBadRequest)
		return
	}

	if order.Symbol == "" {
		order.Symbol = order.Legs[0].Symbol
	}

	c.Lock()
	_ = c.OrderManager.Save(&order)
	c.Unlock()

	msg, err := c.NewOrderMultileg(order)
	if err != nil {
		log.Printf("[ERROR] %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = quickfix.SendToTarget(msg, order.SessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	c.writeOrderJSON(w, &order)
}

func (c tradeClient) updateMultilegOrder(w http.ResponseWriter, r *http.Request) {
	c.Lock()
	defer c.Unlock()

	order, err := c.fetchRequestedOrder(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	var update oms.Order
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&update); err != nil {
		log.Printf("[ERROR] %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := update.Init(); err != nil {
		log.Printf("[ERROR] %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pending := *order
	if update.Quantity != "" {
		pending.Quantity = update.Quantity
		pending.QuantityDecimal = update.QuantityDecimal
	}
	if update.OrdType != "" {
		pending.OrdType = update.OrdType
	}
	if update.Price != "" {
		pending.Price = update.Price
		pending.PriceDecimal = update.PriceDecimal
	}

	clOrdID := c.AssignNextClOrdID(order)

	msg, err := c.MultilegOrderCancelReplace(pending, clOrdID)
	if err != nil {
		log.Printf("[ERROR] %v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = quickfix.SendToTarget(msg, order.SessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	order.ClOrdID = clOrdID
	c.writeOrderJSON(w, order)
}

func main() {
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

	symbolsCfgPath := path.Join("config", "symbols.json")
	symbolsCfg, err := loadSymbolsConfig(symbolsCfgPath)
	if err != nil {
		fmt.Printf("Error loading symbols config %v: %v\n", symbolsCfgPath, err)
		return
	}
	log.Printf("Loaded %d symbols from %s\n", len(symbolsCfg.Symbols), symbolsCfgPath)

	logFactory := NewFancyLog()

	app := newTradeClient(basic.FIXFactory{}, new(alpaca.ClOrdIDGenerator), symbolsCfg)
	fixApp := &basic.FIXApplication{
		SessionIDs:   app.SessionIDs,
		OrderManager: app.OrderManager,
	}
	app.statusSrc = fixApp

	initiator, err := quickfix.NewInitiator(fixApp, quickfix.NewMemoryStoreFactory(), appSettings, logFactory)
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
	router.HandleFunc("/orders/{id:[0-9]+}", app.updateOrder).Methods("PUT")
	router.HandleFunc("/orders/{id:[0-9]+}", app.deleteOrder).Methods("DELETE")

	router.HandleFunc("/executions", app.getExecutions).Methods("GET")
	router.HandleFunc("/executions/{id:[0-9]+}", app.getExecution).Methods("GET")

	router.HandleFunc("/multileg-orders", app.newMultilegOrder).Methods("POST")
	router.HandleFunc("/multileg-orders/{id:[0-9]+}", app.updateMultilegOrder).Methods("PUT")
	router.HandleFunc("/multileg-orders/{id:[0-9]+}", app.deleteOrder).Methods("DELETE")
	router.HandleFunc("/securitydefinitionrequest", app.newSecurityDefintionRequest).Methods("POST")
	router.HandleFunc("/session-status", app.getSessionStatus).Methods("GET")

	router.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", http.FileServer(http.Dir("assets"))))
	router.HandleFunc("/", app.traderView)

	addr := ":" + *port
	log.Printf("Listening on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, router))
}
