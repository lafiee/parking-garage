// Parking garrage simulator
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

type CONFIG struct {
	GARAGE_CAPACITY int `json:"GARAGE_CAPACITY"`
	MAX_ENTRY_WAIT  int `json:"MAX_ENTRY_WAIT"`
	MAX_EXIT_WAIT   int `json:"MAX_EXIT_WAIT"`
}

// Car registered at entrance toll
// "id": <identifier for the event>,
// "vehicle_plate": <alphanumeric registration id of the vehicle>,
// "entry_date_time": <date time in UTC>
type entryEvent struct {
	Id            string `json:"id"`
	VehiclePlate  string `json:"vehicle_plate"`
	EntryDateTime string `json:"entry_date_time"`
}

// Car registered at exit toll
//
//	"id": <identifier for the event>,
//	"vehicle_plate": <alphanumeric registration id of the vehicle>,
//	"exit_date_time": <date time in UTC>,
type exitEvent struct {
	Id           string `json:"id"`
	VehiclePlate string `json:"vehicle_plate"`
	ExitDateTime string `json:"exit_date_time"`
}
type mqttWrapper interface {
	publishEntryEvent([]byte)
	publishExitEvent([]byte)
}

func main() {
	rabbitmqHost := os.Getenv("RABBITMQ_HOST")
	rabbitmqPort := os.Getenv("RABBITMQ_PORT")
	if rabbitmqHost == "" || rabbitmqPort == "" {
		log.Fatalf("RABBITMQ_HOST and RABBITMQ_PORT must be set")
	}
	rabbitmqURL := fmt.Sprintf("amqp://guest:guest@%s:%s/", rabbitmqHost, rabbitmqPort)

	rabbitmq, err := newRabbitClient(rabbitmqURL)
	if err != nil {
		log.Fatalf("Failed to create RabbitMQ client: %s", err)
	}

	defer rabbitmq.conn.Close()
	defer rabbitmq.ch.Close()

	noise := realNoise{}

	config := loadConfig()

	runServices(noise, &rabbitmq, config)

	// Run endlessly
	select {}
}

func runServices(noise randomNoiser, mqtt mqttWrapper, config CONFIG) {
	mutex := &sync.Mutex{}
	parkingLot := []string{}

	go enterTollSimulator(noise, mutex, &parkingLot, mqtt, config)
	go exitTollSimulator(mutex, &parkingLot, mqtt, config)
}

func enterTollSimulator(randomNoise randomNoiser, mutex *sync.Mutex, parkingLot *[]string, mqtt mqttWrapper, config CONFIG) {
	for {
		time.Sleep(time.Duration(rand.Intn(config.MAX_ENTRY_WAIT)+1) * time.Second)

		mutex.Lock()

		// Let car in if there is space
		if len(*parkingLot) < config.GARAGE_CAPACITY {
			enterTollFunc(randomNoise, parkingLot, mqtt)
		}
		mutex.Unlock()
	}
}

func enterTollFunc(randomNoise randomNoiser, parkingLot *[]string, mqtt mqttWrapper) {
	// Randomly generate a car
	vehiclePlate := generateVehiclePlate()
	*parkingLot = append(*parkingLot, vehiclePlate)

	// Random noise that potentially blocks the toll registering the car and not sending the MQTT message
	if randomNoise.noise() {
		entryEvent := entryEvent{uuid.New().String(), vehiclePlate, time.Now().UTC().String()}
		log.Println("incoming:", entryEvent)
		body, err := json.Marshal(entryEvent)
		if err != nil {
			log.Println("Failed to marshal entry-event", err)
		}

		mqtt.publishEntryEvent(body)
	}

}

func exitTollSimulator(mutex *sync.Mutex, parkingLot *[]string, mqtt mqttWrapper, config CONFIG) {
	for {
		time.Sleep(time.Duration(rand.Intn(config.MAX_EXIT_WAIT)+1) * time.Second)

		mutex.Lock()

		// Let car out if there is a car in the parking lot
		if len(*parkingLot) > 0 {
			exitTollFunc(parkingLot, mqtt)
		}
		mutex.Unlock()
	}
}

func exitTollFunc(parkingLot *[]string, mqtt mqttWrapper) {
	carIndex := rand.Intn(len(*parkingLot))

	exitEvent := exitEvent{uuid.New().String(), (*parkingLot)[carIndex], time.Now().UTC().String()}
	*parkingLot = slices.Delete(*parkingLot, carIndex, carIndex+1)

	log.Println("outgoing:", exitEvent)
	body, err := json.Marshal(exitEvent)
	if err != nil {
		log.Println("Failed to marshal exit-event", err)
	}

	mqtt.publishExitEvent(body)
}

func (r *rabbitmqWrapper) publishEntryEvent(body []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := r.ch.PublishWithContext(ctx,
		"",            // exchange
		r.entryQ.Name, // routing key
		false,         // mandatory
		false,         // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        body,
		})
	if err != nil {
		log.Println("Failed to publish entry-event", err)
	}
}

func (r *rabbitmqWrapper) publishExitEvent(body []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := r.ch.PublishWithContext(ctx,
		"",           // exchange
		r.exitQ.Name, // routing key
		false,        // mandatory
		false,        // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        body,
		})
	if err != nil {
		log.Println("Failed to publish exit-event", err)
	}
}

func generateVehiclePlate() string {
	const letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	const numbers = "0123456789"

	plate := make([]byte, 6)
	for i := 0; i < 3; i++ {
		plate[i] = letters[rand.Intn(len(letters))]
	}
	for i := 3; i < 6; i++ {
		plate[i] = numbers[rand.Intn(len(numbers))]
	}

	return string(plate)
}

// 6. 80% of the exit events generated should match a vehicle plate that has a corresponding entry event.
// Remianing exit events should have no correspnding entry events.
// This is to simulate the case where the mall camera did not record vehicle entry event,
// perhaps because one or more of the vehicle plates were not clean/clear enough!
type randomNoiser interface {
	noise() bool
}

type realNoise struct{}

func (r realNoise) noise() bool {
	return rand.Intn(100) < 80
}

func loadConfig() CONFIG {
	configFile, err := os.Open("config/config.json")
	if err != nil {
		log.Fatalf("Failed to open config file: %s", err)
	}
	defer configFile.Close()

	var config CONFIG
	err = json.NewDecoder(configFile).Decode(&config)
	if err != nil {
		log.Fatalf("Failed to decode config file: %s", err)
	}

	return config
}
