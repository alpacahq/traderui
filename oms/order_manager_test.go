package oms

import "testing"

type fakeIDGen struct{}

func (fakeIDGen) Next() string { return "id" }

func TestSaveMessageKeepsOnlyRecent(t *testing.T) {
	om := NewOrderManager(fakeIDGen{})

	total := MaxMessages + 10
	for i := 0; i < total; i++ {
		om.SaveMessage(&Message{})
	}

	got := om.GetAllMessages()
	if len(got) != MaxMessages {
		t.Fatalf("got %d messages, want %d", len(got), MaxMessages)
	}
	for _, m := range got {
		if m.ID <= total-MaxMessages {
			t.Fatalf("message %d should have been evicted", m.ID)
		}
	}
}
