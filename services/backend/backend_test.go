package main

import (
	"net/http"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

type mapDatabase struct {
	storage map[string]entryEvent
}

func (m *mapDatabase) store(entryEvent entryEvent) {
	m.storage[entryEvent.VehiclePlate] = entryEvent
}

func (m *mapDatabase) get(vehiclePlate string) (entryEvent, bool) {
	entryEvent, ok := m.storage[vehiclePlate]
	return entryEvent, ok
}

type mockHTTPClient struct{}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: 200}, nil
}

func TestEntryEventFunc(t *testing.T) {
	database := &mapDatabase{storage: map[string]entryEvent{}}
	body := []byte(`{"id":"1","vehicle_plate":"ABC123","entry_date_time":"2021-01-01T00:00:00Z"}`)
	entryQ := amqp.Delivery{Body: body}

	entryEventFunc(entryQ, database)

	if _, ok := database.get("ABC123"); !ok {
		t.Errorf("Failed to store entry event")
	}
}

func TestExitEventFunc(t *testing.T) {
	database := &mapDatabase{storage: map[string]entryEvent{"ABC123": {Id: "1", VehiclePlate: "ABC123", EntryDateTime: "2021-01-01T00:00:00Z"}}}
	body := []byte(`{"id":"1","vehicle_plate":"ABC123","exit_date_time":"2021-01-01T00:00:00Z"}`)
	exitQ := amqp.Delivery{Body: body}
	httpClient := &mockHTTPClient{}

	exitEventFunc(exitQ, database, httpClient, "")
}
