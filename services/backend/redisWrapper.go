package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisWrapper struct {
	client *redis.Client
}

func (r *redisWrapper) store(entry entryEvent) {
	ctx := context.Background()
	bytes, err := json.Marshal(entry)
	if err != nil {
		log.Fatalf("Failed to marshal entry event: %s", err)
	}

	err = r.client.Set(ctx, entry.VehiclePlate, bytes, 0).Err()
	if err != nil {
		log.Fatalf("Failed to store entry event: %s", err)
	}
}

func (r *redisWrapper) get(vehiclePlate string) (entryEvent, bool) {
	ctx := context.Background()
	entryEvent := entryEvent{}
	val, err := r.client.Get(ctx, vehiclePlate).Result()
	if err != nil {
		log.Println("Failed to get entry event: ", err)
		return entryEvent, false
	}
	json.Unmarshal([]byte(val), &entryEvent)
	return entryEvent, true
}

func createRedisClient(redisURL string) (*redis.Client, error) {
	options := &redis.Options{Addr: redisURL}
	client := redis.NewClient(options)
	ctx := context.Background()

	// Ping the Redis server to check if it's up
	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}
	return client, nil
}

func retryCreateRedisClient(redisURL string) (*redis.Client, error) {
	var client *redis.Client
	var err error
	interval := 1 * time.Second

	for i := 0; i < 15; i++ {
		client, err = createRedisClient(redisURL)
		if err == nil {
			return client, nil
		}

		log.Printf("Failed to connect to Redis: %v. Retrying in %v...", err, interval)
		time.Sleep(interval)
		interval *= 2
	}

	return nil, fmt.Errorf("could not connect to Redis after %d attempts: %v", 15, err)
}
