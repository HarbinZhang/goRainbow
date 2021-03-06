package pipeline

import (
	"os"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"go.uber.org/zap"

	"github.com/harbinzhang/goRainbow/core/module"
	"github.com/harbinzhang/goRainbow/core/util"
)

// Producer send metrics to Kafka
type Producer struct {
	ProduceQueue chan string
	CountService *module.CountService
	Logger       *zap.Logger
}

// Start is a general start
func (p *Producer) Start() {
	defer p.Logger.Sync()

	contextProvider := util.ContextProvider{}
	contextProvider.Init()
	postfix := contextProvider.GetPostfix()
	conf := contextProvider.GetConf()

	kafkaConfig := kafka.ConfigMap{
		"batch.num.messages": 2000,
		"linger.ms":          1,
		"bootstrap.servers":  conf.Kafka.BrokerServers,
		// "buffer.memory=33554432"
		"socket.send.buffer.bytes": 1024000,
		// "reconnect.backoff.ms":     100,
		// "block.on.buffer.full=true"
		"retries":            6,
		"retry.backoff.ms":   100,
		"compression.type":   "gzip",
		"request.timeout.ms": 900000,
	}
	kafkaProducer, err := kafka.NewProducer(&kafkaConfig)
	if err != nil {
		panic("Err building kafka producer: " + err.Error())
	}

	defer kafkaProducer.Close()

	// Delivery report handler for produced messages
	go func() {
		for e := range kafkaProducer.Events() {
			switch ev := e.(type) {
			case *kafka.Message:
				if ev.TopicPartition.Error != nil {
					p.Logger.Warn("Delivery failed",
						zap.String("topicPartition", ev.TopicPartition.String()),
						zap.Int64("timestamp", time.Now().Unix()),
					)
				}
			}
		}
	}()

	// rcsMetricsSent is for metrics level traffic, how many metrics sent to wavefront
	rcsMetricsSent := &util.RequestCounter{
		Name:         "metricsSent",
		Interval:     60 * time.Second,
		ProducerChan: p.ProduceQueue,
		Postfix:      postfix,
	}
	rcsMetricsSent.Init()

	// message := "fjord.burrow.test3.python-consumer-1.BusinessEvent.0.maxLag 0.00 1541214139 source=192.168.3.169 data_center=slv dca_zone=local department=fjord planet=sbx888 service_name=porter_rainbow porter_tools=porter-rainbow"

	// Wait for message deliveries before shutting down
	go kafkaProducer.Flush(15 * 1000)

	// Produce messages to topic (asynchronously)
	topic := conf.Kafka.Topic

	env := os.Getenv("ENV")

	for message := range p.ProduceQueue {
		go rcsMetricsSent.Increase(env)
		p.Logger.Debug("Produced to speed-racer: " + message)
		kafkaProducer.Produce(&kafka.Message{
			TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
			Value:          []byte(message),
		}, nil)
	}
}

// Stop is a general stop
func (p *Producer) Stop() error {
	return nil
}
