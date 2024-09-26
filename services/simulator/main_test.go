package main

import "testing"

type mockNoise struct{}

func (m mockNoise) noise() bool {
	return true
}

type mockMqtt struct{}

func (m mockMqtt) publishEntryEvent([]byte) {}
func (m mockMqtt) publishExitEvent([]byte)  {}

func TestEnterTollFunc(t *testing.T) {
	mockNoise := mockNoise{}
	mockMqtt := mockMqtt{}
	parkingLot := []string{}
	enterTollFunc(mockNoise, &parkingLot, mockMqtt)

	if len(parkingLot) == 0 {
		t.Errorf("Expected parkingLot to have a car")
	}
}

func TestExitTollFunc(t *testing.T) {
	mockMqtt := mockMqtt{}
	parkingLot := []string{"ABC123"}
	exitTollFunc(&parkingLot, mockMqtt)

	if len(parkingLot) != 0 {
		t.Errorf("Expected parkingLot to be empty")
	}
}

func TestLoadConfig(t *testing.T) {
	config := loadConfig()
	if config.GARAGE_CAPACITY == 0 || config.MAX_ENTRY_WAIT == 0 || config.MAX_EXIT_WAIT == 0 {
		t.Errorf("Expected non-zero values for GARAGE_CAPACITY, MAX_ENTRY_WAIT, and MAX_EXIT_WAIT")
	}
}
