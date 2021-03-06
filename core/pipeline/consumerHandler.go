package pipeline

import (
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/harbinzhang/goRainbow/core/module"
	"github.com/harbinzhang/goRainbow/core/protocol"
	"github.com/harbinzhang/goRainbow/core/util"
)

// ConsumerHandler handles:
// 1. consumer offset
// 2. consumer total lag
// 3. consumer partition lag
// 4. consumer max lag of partition
// 5. consumer offset change rate
type ConsumerHandler struct {
	ProduceQueue       chan string
	CountService       *module.CountService
	Logger             *zap.Logger
	ClusterConsumerMap *util.SyncNestedMap

	consumersLink string
	consumer      string
	cluster       string
}

// Init is a general init
func (ch *ConsumerHandler) Init(consumersLink string, consumer string, cluster string) {
	ch.consumersLink = consumersLink
	ch.consumer = consumer
	ch.cluster = cluster
}

// Start is a general start
func (ch *ConsumerHandler) Start() {
	defer ch.Logger.Sync()

	fmt.Println("New consumer found: ", ch.consumersLink, ch.consumer)

	lagInfoQueue := make(chan protocol.LagInfo)

	ticker := time.NewTicker(30 * time.Second)

	prefix := "fjord.burrow." + ch.cluster + "." + ch.consumer

	translator := &Translator{
		LagQueue:     lagInfoQueue,
		ProduceQueue: ch.ProduceQueue,
		CountService: ch.CountService,
		Logger: util.GetLogger().With(
			zap.String("module", "Translator"),
		),
	}
	translator.Init(prefix, ch.cluster)
	go translator.Start()

	for {
		// check its ch.consumer lag from Burrow periodically
		<-ticker.C
		var lagInfo protocol.LagInfo
		getHTTPStruct(ch.consumersLink+ch.consumer+"/lag", &lagInfo.Lag)
		if lagInfo.Lag.Error {
			ch.Logger.Warn("Get consumer /lag error",
				zap.String("message", lagInfo.Lag.Message),
				zap.Int64("timestamp", time.Now().Unix()),
			)
			break
		}
		lagInfo.Timestamp = time.Now().Unix()
		lagInfoQueue <- lagInfo
	}

	// snm.DeregisterChild(cluster, ch.consumer)
	ch.ClusterConsumerMap.SetLock(ch.cluster)
	delete(ch.ClusterConsumerMap.GetChild(ch.cluster, nil).(map[string]interface{}), ch.consumer)
	ch.ClusterConsumerMap.ReleaseLock(ch.cluster)

	close(lagInfoQueue)
	ch.Logger.Warn("consumer is invalid, will stop handler.",
		zap.String("consumer", ch.consumer),
		zap.String("cluster", ch.cluster),
		zap.Int64("timestamp", time.Now().Unix()),
	)
}

// Stop is a general stop
func (ch *ConsumerHandler) Stop() error {
	return nil
}
