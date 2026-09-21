package messaging

import "github.com/rabbitmq/amqp091-go"

func NewRabbitMQ(url string) (*amqp091.Connection, error) {
	return amqp091.Dial(url)
}
