package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-kafka/v2/pkg/kafka"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/message/router/middleware"
	"github.com/ThreeDotsLabs/watermill/message/router/plugin"
)

var (
	brokers      = []string{"kafka:9092"}
	consumeTopic = "events"
	publishTopic = "events-processed"

	logger = watermill.NewStdLogger(
		true,  // debug
		false, // route-tracing
	)
	marshaler = kafka.DefaultMarshaler{}
)

type event struct {
	ID int `json:"id"`
}

type processedEvent struct {
	ProcessedID int       `json:"processed_id"`
	Time        time.Time `json:"time"`
}

func main() {
	publisher := createPublisher()

	subscriber := createSubscriber("handler_1")

	router, err := message.NewRouter(message.RouterConfig{}, logger)
	if err != nil {
		panic(err)
	}

	router.AddPlugin(plugin.SignalsHandler)
	router.AddMiddleware(middleware.Recoverer)
	router.AddHandler(
		"handler_1",
		consumeTopic,
		subscriber,
		publishTopic,
		publisher,
		func(msg *message.Message) ([]*message.Message, error) {
			consumedPayload := event{}
			err := json.Unmarshal(msg.Payload, &consumedPayload)
			if err != nil {
				return nil, err
			}

			log.Printf("recieved event %+v", consumedPayload)

			newPayload, err := json.Marshal(processedEvent{
				ProcessedID: consumedPayload.ID,
				Time:        time.Now(),
			})
			if err != nil {
				return nil, err
			}

			newMessage := message.NewMessage(watermill.NewUUID(), newPayload)

			return []*message.Message{newMessage}, nil
		},
	)

	go simulateEvents(publisher)

	if err := router.Run(context.Background()); err != nil {
		panic(err)
	}
}

func createPublisher() message.Publisher {
	kafkaPublisher, err := kafka.NewPublisher(
		kafka.PublisherConfig{
			Brokers:   brokers,
			Marshaler: marshaler,
		},
		logger,
	)

	if err != nil {
		panic(err)
	}

	return kafkaPublisher
}

func createSubscriber(consumerGroup string) message.Subscriber {
	kafkaSubscriber, err := kafka.NewSubscriber(
		kafka.SubscriberConfig{
			Brokers:       brokers,
			Unmarshaler:   marshaler,
			ConsumerGroup: consumerGroup,
		},
		logger,
	)
	if err != nil {
		panic(err)
	}

	return kafkaSubscriber
}

func simulateEvents(publisher message.Publisher) {
	i := 0
	for {
		e := event{
			ID: i,
		}

		payload, err := json.Marshal(e)
		if err != nil {
			panic(err)
		}

		err = publisher.Publish(consumeTopic, message.NewMessage(
			watermill.NewUUID(),
			payload,
		))

		if err != nil {
			panic(err)
		}

		i++

		time.Sleep(time.Second)
	}
}
