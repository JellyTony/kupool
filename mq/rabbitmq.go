package mq

import (
	"context"
	"encoding/json"
	"time"

	"github.com/JellyTony/kupool/events"
	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQ struct {
	conn *amqp.Connection
	ch   *amqp.Channel
	q    amqp.Queue
	out  chan events.SubmitEvent
}

func NewRabbitMQ(url, queue string) (*RabbitMQ, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}
	_ = ch.Confirm(false)
	q, err := ch.QueueDeclare(queue, true, false, false, false, nil)
	if err != nil {
		return nil, err
	}
	r := &RabbitMQ{conn: conn, ch: ch, q: q, out: make(chan events.SubmitEvent, 1024)}
	go r.consume()
	return r, nil
}

func (r *RabbitMQ) Publish(evt events.SubmitEvent) error {
	b, _ := json.Marshal(evt)
	return r.ch.PublishWithContext(context.Background(), "", r.q.Name, false, false, amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
		ContentType:  "application/json",
		Body:         b,
	})
}

func (r *RabbitMQ) consume() {
	msgs, err := r.ch.Consume(r.q.Name, "", false, false, false, false, nil)
	if err != nil {
		close(r.out)
		return
	}
	for m := range msgs {
		var evt events.SubmitEvent
		if json.Unmarshal(m.Body, &evt) == nil {
			r.out <- evt
			_ = m.Ack(false)
		} else {
			_ = m.Nack(false, false)
		}
	}
}

func (r *RabbitMQ) Subscribe() <-chan events.SubmitEvent { return r.out }

func (r *RabbitMQ) Close() error {
	close(r.out)
	if r.ch != nil {
		_ = r.ch.Close()
	}
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}
