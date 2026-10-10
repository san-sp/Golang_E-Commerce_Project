package messaging

import (
	"errors"
	"testing"

	"github.com/rabbitmq/amqp091-go"
)

type fakeAcknowledger struct {
	ackCalls    int
	nackCalls   int
	rejectCalls int

	lastTag      uint64
	lastMultiple bool
	lastRequeue  bool

	ackErr  error
	nackErr error
}

func (f *fakeAcknowledger) Ack(tag uint64, multiple bool) error {
	f.ackCalls++
	f.lastTag = tag
	f.lastMultiple = multiple
	return f.ackErr
}

func (f *fakeAcknowledger) Nack(tag uint64, multiple, requeue bool) error {
	f.nackCalls++
	f.lastTag = tag
	f.lastMultiple = multiple
	f.lastRequeue = requeue
	return f.nackErr
}

func (f *fakeAcknowledger) Reject(tag uint64, requeue bool) error {
	f.rejectCalls++
	f.lastTag = tag
	f.lastRequeue = requeue
	return nil
}

type fakeConsumerChannel struct {
	messages chan amqp091.Delivery
}

func (f *fakeConsumerChannel) Qos(
	prefetchCount int,
	prefetchSize int,
	global bool,
) error {
	return nil
}

func (f *fakeConsumerChannel) Consume(
	queue string,
	consumer string,
	autoAck bool,
	exclusive bool,
	noLocal bool,
	noWait bool,
	args amqp091.Table,
) (<-chan amqp091.Delivery, error) {
	return f.messages, nil
}

func TestValidateHandlerResult(t *testing.T) {
	testErr := errors.New("processing failed")

	tests := []struct {
		name    string
		result  ProcessingResult
		err     error
		wantErr bool
	}{
		{
			name:    "success without error",
			result:  ProcessingSuccess,
			err:     nil,
			wantErr: false,
		},
		{
			name:    "success with error",
			result:  ProcessingSuccess,
			err:     testErr,
			wantErr: true,
		},
		{
			name:    "retry with error",
			result:  ProcessingRetry,
			err:     testErr,
			wantErr: false,
		},
		{
			name:    "reject with error",
			result:  ProcessingReject,
			err:     testErr,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateHandlerResult(
				tt.result,
				tt.err,
			)

			if (err != nil) != tt.wantErr {
				t.Fatalf(
					"expected error=%v, got error=%v",
					tt.wantErr,
					err,
				)
			}
		})
	}
}

func runConsumerTest(
	handler MessageHandler,
	retryHandler RetryHandler,
) (*fakeAcknowledger, error) {
	acknowledger := &fakeAcknowledger{}
	messages := make(chan amqp091.Delivery, 1)

	messages <- amqp091.Delivery{
		Acknowledger: acknowledger,
		DeliveryTag:  42,
	}
	close(messages)

	channel := &fakeConsumerChannel{
		messages: messages,
	}
	consumer := &Consumer{
		channel: channel,
	}

	err := consumer.Start("test-queue", handler, retryHandler)

	return acknowledger, err
}

func TestConsumerStart(t *testing.T) {
	testErr := errors.New("processing failed")

	tests := []struct {
		name           string
		result         ProcessingResult
		handlerErr     error
		retryErr       error
		wantErr        bool
		wantAck        int
		wantNack       int
		wantRequeue    bool
		wantRetryCalls int
	}{
		{
			name:     "success acknowledges message",
			result:   ProcessingSuccess,
			wantAck:  1,
			wantNack: 0,
		},
		{
			name:           "retry republishes then acknowledges message",
			result:         ProcessingRetry,
			wantAck:        1,
			wantNack:       0,
			wantRetryCalls: 1,
		},
		{
			name:           "retry failure leaves message unacknowledged",
			result:         ProcessingRetry,
			retryErr:       testErr,
			wantErr:        true,
			wantAck:        0,
			wantNack:       0,
			wantRetryCalls: 1,
		},
		{
			name:        "reject negatively acknowledges without requeue",
			result:      ProcessingReject,
			wantAck:     0,
			wantNack:    1,
			wantRequeue: false,
		},
		{
			name:     "invalid result returns error",
			result:   ProcessingResult(99),
			wantErr:  true,
			wantAck:  0,
			wantNack: 0,
		},
		{
			name:       "success with error returns error without acknowledgement",
			result:     ProcessingSuccess,
			handlerErr: testErr,
			wantErr:    true,
			wantAck:    0,
			wantNack:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retryCalls := 0

			acknowledger, err := runConsumerTest(
				func(amqp091.Delivery) (ProcessingResult, error) {
					return tt.result, tt.handlerErr
				},
				func(amqp091.Delivery) error {
					retryCalls++
					return tt.retryErr
				},
			)

			if retryCalls != tt.wantRetryCalls {
				t.Errorf(
					"expected %d retry calls, got %d",
					tt.wantRetryCalls,
					retryCalls,
				)
			}

			if (err != nil) != tt.wantErr {
				t.Fatalf("expected error=%v, got %v", tt.wantErr, err)
			}

			if acknowledger.ackCalls != tt.wantAck {
				t.Errorf(
					"expected %d ACK calls, got %d",
					tt.wantAck,
					acknowledger.ackCalls,
				)
			}

			if acknowledger.nackCalls != tt.wantNack {
				t.Errorf(
					"expected %d NACK calls, got %d",
					tt.wantNack,
					acknowledger.nackCalls,
				)
			}

			if acknowledger.ackCalls > 0 {
				if acknowledger.lastTag != 42 {
					t.Errorf(
						"expected delivery tag 42, got %d",
						acknowledger.lastTag,
					)
				}

				if acknowledger.lastMultiple {
					t.Error("expected ACK multiple=false")
				}
			}

			if acknowledger.nackCalls > 0 && acknowledger.lastRequeue != tt.wantRequeue {
				t.Errorf(
					"expected NACK requeue=%v, got %v",
					tt.wantRequeue,
					acknowledger.lastRequeue,
				)
			}
		})
	}
}
