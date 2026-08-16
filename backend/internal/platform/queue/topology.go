package queue

import (
	"time"

	"github.com/rabbitmq/amqp091-go"
)

type QueueNames struct {
	Main  string
	Retry string
	Dead  string
}

func Names(main string) QueueNames {
	return QueueNames{Main: main, Retry: main + ".retry", Dead: main + ".dead"}
}

func RetryQueueArguments(main string, delay time.Duration) amqp091.Table {
	return amqp091.Table{
		"x-message-ttl":             int32(delay / time.Millisecond),
		"x-dead-letter-exchange":    "",
		"x-dead-letter-routing-key": main,
	}
}
