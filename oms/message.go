package oms

// Message is an inbound FIX application message kept for display.
type Message struct {
	ID      int    `json:"id"`
	Session string `json:"session_id"`
	MsgType string `json:"msg_type"`
	Raw     string `json:"raw"`
}
