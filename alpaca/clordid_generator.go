package alpaca

import "github.com/google/uuid"

type ClOrdIDGenerator struct{}

func (f *ClOrdIDGenerator) Next() string {
	return uuid.New().String()
}
