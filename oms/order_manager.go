package oms

import (
	"fmt"
	"sync"
)

// MaxMessages is how many recent inbound messages the OrderManager retains.
const MaxMessages = 500

type ClOrdIDGenerator interface {
	Next() string
}

type OrderManager struct {
	sync.RWMutex
	orderID     int
	executionID int
	messageID   int
	clOrdID     ClOrdIDGenerator

	orders        map[int]*Order
	clOrdIDLookup map[string]*Order
	executions    map[int]*Execution
	messages      map[int]*Message
}

func NewOrderManager(idGen ClOrdIDGenerator) *OrderManager {
	return &OrderManager{
		clOrdIDLookup: make(map[string]*Order),
		orders:        make(map[int]*Order),
		executions:    make(map[int]*Execution),
		messages:      make(map[int]*Message),
		clOrdID:       idGen,
	}
}

func (om *OrderManager) GetAll() []*Order {
	orders := make([]*Order, 0, len(om.clOrdIDLookup))
	for _, v := range om.clOrdIDLookup {
		orders = append(orders, v)
	}

	return orders
}

func (om *OrderManager) GetAllExecutions() []*Execution {
	executions := make([]*Execution, 0, len(om.executions))
	for _, v := range om.executions {
		executions = append(executions, v)
	}

	return executions
}

func (om *OrderManager) GetAllMessages() []*Message {
	messages := make([]*Message, 0, len(om.messages))
	for _, v := range om.messages {
		messages = append(messages, v)
	}

	return messages
}

func (om *OrderManager) Get(id int) (*Order, error) {
	var err error
	order, ok := om.orders[id]
	if !ok {
		err = fmt.Errorf("could not find order with id %v", id)
	}

	return order, err
}

func (om *OrderManager) GetExecution(id int) (*Execution, error) {
	var err error
	exec, ok := om.executions[id]
	if !ok {
		err = fmt.Errorf("could not find execution with id %v", id)
	}

	return exec, err
}

func (om *OrderManager) GetByClOrdID(clOrdID string) (*Order, error) {
	var err error
	order, ok := om.clOrdIDLookup[clOrdID]
	if !ok {
		err = fmt.Errorf("could not find order with clordid %v", clOrdID)
	}

	return order, err
}

func (om *OrderManager) Save(order *Order) error {
	order.ID = om.nextOrderID()
	order.ClOrdID = om.clOrdID.Next()

	om.orders[order.ID] = order
	om.clOrdIDLookup[order.ClOrdID] = order

	return nil
}

func (om *OrderManager) SaveExecution(exec *Execution) error {
	exec.ID = om.nextExecutionID()
	om.executions[exec.ID] = exec

	return nil
}

// SaveMessage records an inbound message, keeping only the most recent
// MaxMessages so a long-running session does not grow without bound.
func (om *OrderManager) SaveMessage(msg *Message) {
	om.messageID++
	msg.ID = om.messageID
	om.messages[msg.ID] = msg
	delete(om.messages, msg.ID-MaxMessages)
}

// NextClOrdID returns a fresh ID without attaching it to an order.
func (om *OrderManager) NextClOrdID() string {
	return om.clOrdID.Next()
}

func (om *OrderManager) AssignNextClOrdID(order *Order) string {
	clOrdID := om.clOrdID.Next()
	om.clOrdIDLookup[clOrdID] = order
	return clOrdID
}

func (om *OrderManager) nextOrderID() int {
	om.orderID++
	return om.orderID
}

func (om *OrderManager) nextExecutionID() int {
	om.executionID++
	return om.executionID
}
