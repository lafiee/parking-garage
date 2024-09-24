// Parking garrage simulator
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/google/uuid"
)

func chanMain() {
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

	entryChann := enterTollSim(noise, config, &rabbitmq)
	exitChann := waiter(entryChann, config)
	for n := range exitChann {
		exitTollSim(n, &rabbitmq)
	}
}

func enterTollSim(randomNoise randomNoiser, config CONFIG, mqtt mqttWrapper) <-chan entryEvent {
	ch := make(chan entryEvent)
	go func() {
		for i := 0; i < 500; i++ {
			time.Sleep(time.Duration(rand.Intn(config.MAX_ENTRY_WAIT)+1) * time.Second)
			vehiclePlate := generateVehiclePlate()
			entryEvent := entryEvent{uuid.New().String(), vehiclePlate, time.Now().UTC().String()}
			log.Println("incoming:", entryEvent)

			ch <- entryEvent

			if randomNoise.noise() {
				body, err := json.Marshal(entryEvent)
				if err != nil {
					log.Println("Failed to marshal entry-event", err)
				}

				mqtt.publishEntryEvent(body)
			}
		}
		close(ch)
	}()
	return ch
}

func waiter(entryChann <-chan entryEvent, config CONFIG) <-chan entryEvent {
	ch := make(chan entryEvent)
	go func() {
		for n := range entryChann {
			go func(e entryEvent) {
				fmt.Println("car started parking", e.VehiclePlate)
				time.Sleep(time.Duration(rand.Intn(config.MAX_EXIT_WAIT)+1) * time.Second)
				fmt.Println("car finished parking and is exiting", e.VehiclePlate)
				ch <- e
			}(n)
		}
		close(ch)
	}()
	return ch
}

func exitTollSim(entryEvent entryEvent, mqtt mqttWrapper) {
	exitEvent := exitEvent{uuid.New().String(), entryEvent.VehiclePlate, time.Now().UTC().String()}
	log.Println("outgoing:", exitEvent)
	body, err := json.Marshal(exitEvent)
	if err != nil {
		log.Println("Failed to marshal exit-event", err)
	}

	mqtt.publishExitEvent(body)
}
