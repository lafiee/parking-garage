package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	amqp "github.com/rabbitmq/amqp091-go"
)

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

type summary struct {
	Vehicle   string `json:"vehicle"`
	EntryTime string `json:"entryTime"`
	ExitTime  string `json:"exitTime"`
}

type databaser interface {
	store(entryEvent)
	get(string) (entryEvent, bool)
}

type httpClienter interface {
	Do(*http.Request) (*http.Response, error)
}

var (
	postRequestLatency = prometheus.NewSummary(prometheus.SummaryOpts{
		Name:       "post_request_latency_seconds",
		Help:       "Latency of POST requests to the writer service in seconds",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	})
)

func init() {
	prometheus.MustRegister(postRequestLatency)
}

func main() {
	rabbitmqHost := os.Getenv("RABBITMQ_HOST")
	rabbitmqPort := os.Getenv("RABBITMQ_PORT")
	if rabbitmqHost == "" || rabbitmqPort == "" {
		log.Fatalf("RABBITMQ_HOST and RABBITMQ_PORT must be set")
	}
	rabbitmqURL := fmt.Sprintf("amqp://guest:guest@%s:%s/", rabbitmqHost, rabbitmqPort)

	writerHost := os.Getenv("WRITER_HOST")
	writerPort := os.Getenv("WRITER_PORT")
	if writerHost == "" || writerPort == "" {
		log.Fatalf("WRITER_HOST and WRITER_PORT must be set")
	}
	writerURL := fmt.Sprintf("http://%s:%s/log", writerHost, writerPort)

	redisHost := os.Getenv("REDIS_HOST")
	redisPort := os.Getenv("REDIS_PORT")
	if redisHost == "" || redisPort == "" {
		log.Fatalf("REDIS_HOST and REDIS_PORT must be set")
	}
	redisURL := fmt.Sprintf("%s:%s", redisHost, redisPort)

	var conn *amqp.Connection
	var err error
	for i := 0; ; i++ {
		conn, err = amqp.Dial(rabbitmqURL)
		if err != nil {
			fmt.Println("Failed to connect to RabbitMQ", err)
		}

		if conn != nil {
			break
		}
		time.Sleep(1 * time.Second)

		if i == 15*60 {
			return
		}

	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("%s: %s", "Failed to open a channel", err)
	}
	defer ch.Close()

	entryQ, err := ch.QueueDeclare(
		"entry-event", // name
		false,         // durable
		false,         // delete when unused
		false,         // exclusive
		false,         // no-wait
		nil,           // arguments
	)
	if err != nil {
		log.Fatalf("%s: %s", "Failed to declare entry queue", err)
	}

	entryMsgs, err := ch.Consume(
		entryQ.Name, // queue
		"",          // consumer
		true,        // auto-ack
		false,       // exclusive
		false,       // no-local
		false,       // no-wait
		nil,         // args
	)
	if err != nil {
		log.Fatalf("%s: %s", "Failed to register a consumer", err)
	}

	exitQ, err := ch.QueueDeclare(
		"exit-event", // name
		false,        // durable
		false,        // delete when unused
		false,        // exclusive
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		log.Fatalf("%s: %s", "Failed to declare exit queue", err)
	}

	exitMsgs, err := ch.Consume(
		exitQ.Name, // queue
		"",         // consumer
		true,       // auto-ack
		false,      // exclusive
		false,      // no-local
		false,      // no-wait
		nil,        // args
	)
	if err != nil {
		log.Fatalf("%s: %s", "Failed to register exit consumer", err)
	}

	// Start prometheus server
	http.Handle("/metrics", promhttp.Handler())
	prometheusMetricsPort := os.Getenv("PROMETHEUS_METRICS_PORT")
	if prometheusMetricsPort == "" {
		log.Fatalf("PROMETHEUS_METRICS_PORT must be set")
	}
	go http.ListenAndServe(":"+prometheusMetricsPort, nil)

	httpClient := &http.Client{Timeout: 10 * time.Second}

	redis, err := retryCreateRedisClient(redisURL)
	if err != nil {
		log.Fatalln("Failed to connect to Redis: ", err)
	}
	database := &redisWrapper{client: redis}

	go consumeEntryEvents(entryMsgs, database)
	go consumeExitEvents(exitMsgs, database, httpClient, writerURL)

	select {}
}

func consumeEntryEvents(delivery <-chan amqp.Delivery, database databaser) {
	for d := range delivery {
		entryEventFunc(d, database)
	}
}

func entryEventFunc(d amqp.Delivery, database databaser) {
	log.Printf("Received evntry event: %s", d.Body)

	entryEvent := entryEvent{}
	err := json.Unmarshal(d.Body, &entryEvent)
	if err != nil {
		log.Fatalf("Failed to unmarshal entry event: %s", err)
	}

	database.store(entryEvent)
}

func consumeExitEvents(delivery <-chan amqp.Delivery, database databaser, httpClient *http.Client, writerURL string) {
	for d := range delivery {
		exitEventFunc(d, database, httpClient, writerURL)
	}
}

func exitEventFunc(d amqp.Delivery, database databaser, httpClient httpClienter, writerURL string) {
	log.Printf("Received exit event: %s", d.Body)

	exitEvent := exitEvent{}
	err := json.Unmarshal(d.Body, &exitEvent)
	if err != nil {
		log.Fatalf("Failed to unmarshal exit event: %s", err)
	}

	entryEvent, ok := database.get(exitEvent.VehiclePlate)
	// We did not manage to register the car's entrance event. For now just give customer the minimum parking time
	if !ok {
		entryEvent.EntryDateTime = exitEvent.ExitDateTime
	}

	summary := summary{
		Vehicle:   exitEvent.VehiclePlate,
		EntryTime: entryEvent.EntryDateTime,
		ExitTime:  exitEvent.ExitDateTime,
	}
	summaryBytes, err := json.Marshal(summary)
	if err != nil {
		log.Fatalf("Failed to marshal summary: %s", err)
	}

	httpRequest, err := http.NewRequest("POST", writerURL, bytes.NewBuffer(summaryBytes))
	if err != nil {
		log.Fatalf("Failed to create HTTP request: %s", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")

	// Lazy retry
	for i := 0; i < 15; i++ {
		start := time.Now()
		httpResponse, err := httpClient.Do(httpRequest)
		duration := time.Since(start).Seconds()
		postRequestLatency.Observe(duration)

		if err != nil {
			log.Println("Failed to send HTTP request: ", err)
		}
		if httpResponse.StatusCode == http.StatusOK {
			break
		}
		time.Sleep(1 * time.Second)
	}
}
