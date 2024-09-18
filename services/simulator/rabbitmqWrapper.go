package main

import (
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type rabbitmqWrapper struct {
	conn   *amqp.Connection
	ch     *amqp.Channel
	entryQ amqp.Queue
	exitQ  amqp.Queue
}

func createRabbitClient(url string) (rabbitmqWrapper, error) {
	rabbitmq := rabbitmqWrapper{}
	var err error
	rabbitmq.conn, err = amqp.Dial(url)
	if err != nil {
		return rabbitmq, err
	}

	rabbitmq.ch, err = rabbitmq.conn.Channel()
	if err != nil {
		rabbitmq.conn.Close()
		return rabbitmq, err
	}

	rabbitmq.entryQ, err = rabbitmq.ch.QueueDeclare(
		"entry-event", // name
		false,         // durable
		false,         // delete when unused
		false,         // exclusive
		false,         // no-wait
		nil,           // arguments
	)
	if err != nil {
		rabbitmq.conn.Close()
		rabbitmq.ch.Close()
		return rabbitmq, err
	}

	rabbitmq.exitQ, err = rabbitmq.ch.QueueDeclare(
		"exit-event", // name
		false,        // durable
		false,        // delete when unused
		false,        // exclusive
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		rabbitmq.conn.Close()
		rabbitmq.ch.Close()
		return rabbitmq, err
	}

	return rabbitmq, nil
}

func newRabbitClient(url string) (rabbitmqWrapper, error) {
	var rabbitmq rabbitmqWrapper
	var err error
	for i := 0; ; i++ {
		rabbitmq, err = createRabbitClient(url)
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)

		if i == 15*60 {
			return rabbitmq, err
		}
	}
	return rabbitmq, nil
}
