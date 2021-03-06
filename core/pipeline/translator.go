package pipeline

import (
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/harbinzhang/goRainbow/core/module"
	"github.com/harbinzhang/goRainbow/core/protocol"
	"github.com/harbinzhang/goRainbow/core/util"
)

// Translator for message translate from struct to string
type Translator struct {
	LagQueue     <-chan protocol.LagInfo
	ProduceQueue chan<- string
	CountService *module.CountService
	Logger       *zap.Logger

	prefix  string
	env     string
	postfix string
	oom     *module.OwnerOffsetMoveHelper
}

// Init is a general init
func (t *Translator) Init(prefix string, env string) {
	t.prefix = prefix
	t.env = env

	contextProvider := util.ContextProvider{}
	contextProvider.Init()
	t.postfix = contextProvider.GetPostfix()

	// Prepare consumer side offset change per minute
	t.oom = &module.OwnerOffsetMoveHelper{
		CountService: t.CountService,
		ProduceQueue: t.ProduceQueue,
		Logger: util.GetLogger().With(
			zap.String("module", "consumerOwnerOffsetMoveHelper"),
		),
	}
	t.oom.Init(t.prefix, t.postfix, t.env, "hosts")
}

// Start is a general start
func (t *Translator) Start() {
	defer t.Logger.Sync()

	for lagInfo := range t.LagQueue {
		go t.parseInfo(lagInfo)
	}

	t.Logger.Warn("translator exit",
		zap.String("prefix", t.prefix),
		zap.Int64("timestamp", time.Now().Unix()),
	)
}

// Stop is a general stop
func (t *Translator) Stop() error {
	return nil
}

func (t *Translator) parseInfo(lagInfo protocol.LagInfo) {
	// lag is 0 or non-zero.
	// parse it into lower level(partitions, maxlag).
	cluster := lagInfo.Lag.Status.Cluster
	group := lagInfo.Lag.Status.Group
	totalLag := strconv.Itoa(lagInfo.Lag.Status.Totallag)
	timestamp := strconv.FormatInt(lagInfo.Timestamp, 10)

	envTag := "env=" + cluster
	consumerTag := "consumer=" + group
	newPostfix := strings.Join([]string{timestamp, t.postfix, envTag, consumerTag}, " ")

	t.CountService.Increase("totalMessage", cluster)

	t.ProduceQueue <- combineInfo([]string{t.prefix, "totalLag"}, []string{totalLag, newPostfix})

	if totalLag != "0" {
		t.CountService.Increase("validMessage", cluster)
	}

	go t.parsePartitionInfo(lagInfo.Lag.Status.Partitions, newPostfix, lagInfo.Timestamp)
	go t.parseMaxLagInfo(lagInfo.Lag.Status.Maxlag, newPostfix)
}

func (t *Translator) parsePartitionInfo(partitions []protocol.Partition, postfix string, timestamp int64) {
	for _, partition := range partitions {

		partitionID := strconv.Itoa(partition.Partition)
		currentLag := partition.CurrentLag

		owner := partition.Owner
		// it happens when info is invalid, skip this info.
		if owner == "" {
			t.Logger.Warn("owner invalid",
				zap.String("prefix", t.prefix),
				zap.Int("currentLag", currentLag),
				zap.String("partitionID", partitionID),
				zap.Int64("timestamp", time.Now().Unix()),
			)
			t.CountService.Increase("exception.ownerInvalid", t.env)
			return
		}

		topic := partition.Topic

		startOffset := strconv.Itoa(partition.Start.Offset)
		// startOffsetTimestamp := strconv.FormatInt(partition.Start.Timestamp, 10)
		endOffset := strconv.Itoa(partition.End.Offset)
		// endOffsetTimestamp := strconv.FormatInt(partition.End.Timestamp, 10)

		t.oom.Update(owner+":"+partitionID, partition.End.Offset, timestamp)

		topicTag := "topic=" + topic
		partitionTag := "partition=" + partitionID
		ownerTag := "owner=" + owner

		// This part code doesn't work.
		// ie. send a 30s before timestamp doesn't work in wavefront.
		// The goal is to send previous "lag=0" to make metric look better.
		// if shouldSendPreviousLag {
		// 	previousTimestamp, err := strconv.ParseInt(strings.Split(postfix, " ")[0], 10, 64)
		// 	previousTimestamp -= 60
		// 	if err != nil {
		// 		log.Println("ERROR: Cannot parse previousTimestamp in shouldSendPreviousLag.")
		// 		return
		// 	}
		// 	produceQueue <- combineInfo([]string{prefix, topic, partitionID, "Lag"}, []string{"0", strconv.FormatInt(previousTimestamp, 10), postfix, topicTag, partitionTag, ownerTag})
		// }

		t.ProduceQueue <- combineInfo([]string{t.prefix, topic, partitionID, "Lag"}, []string{strconv.Itoa(currentLag), postfix, topicTag, partitionTag, ownerTag})
		t.ProduceQueue <- combineInfo([]string{t.prefix, topic, partitionID, "startOffset"}, []string{startOffset, postfix, topicTag, partitionTag, ownerTag})
		t.ProduceQueue <- combineInfo([]string{t.prefix, topic, partitionID, "endOffset"}, []string{endOffset, postfix, topicTag, partitionTag, ownerTag})
	}
}

func (t *Translator) parseMaxLagInfo(maxLag protocol.MaxLag, postfix string) {
	// tags: owner
	// metrics: partitionID, currentLag, startOffset, endOffset, topic

	owner := maxLag.Owner
	// it happens when info is invalid, skip this info.
	if owner == "" {
		t.Logger.Warn("owner invalid",
			zap.String("prefix", t.prefix),
			zap.Int("maxLagPartitionID", maxLag.Partition),
			zap.Int64("timestamp", time.Now().Unix()),
		)
		t.CountService.Increase("exception.ownerInvalid", t.env)
		return
	}
	ownerTag := "owner=" + owner

	// MaxLagPartition Level handle
	maxLagMap := make(map[string]string)
	maxLagMap["maxLagmaxLagPartitionID"] = strconv.Itoa(maxLag.Partition)
	maxLagMap["maxLagCurrentLag"] = strconv.Itoa(maxLag.CurrentLag)
	maxLagMap["maxLagStartOffset"] = strconv.Itoa(maxLag.Start.Offset)
	maxLagMap["maxLagEndOffset"] = strconv.Itoa(maxLag.End.Offset)
	maxLagMap["maxLagTopic"] = maxLag.Topic

	for key, value := range maxLagMap {
		t.ProduceQueue <- combineInfo([]string{t.prefix, key}, []string{value, postfix, ownerTag})
	}
}

func combineInfo(prefix []string, postfix []string) string {
	return strings.Join(prefix, ".") + " " + strings.Join(postfix, " ")
}
