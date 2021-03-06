package util

import (
	"strconv"
	"strings"
	"sync"
	"time"
)

// RequestCounter is for counting events in go-rainbow
type RequestCounter struct {
	sync.Mutex

	Interval     time.Duration
	ProducerChan chan<- string
	Name         string
	Postfix      string

	envCount         map[string]int
	unavailableCount int
	ticker           *time.Ticker
	quitChannel      chan struct{}
}

// Init is to initial a RequestCounter
func (rc *RequestCounter) Init() {
	rc.envCount = make(map[string]int)
	rc.unavailableCount = 0

	rc.quitChannel = make(chan struct{})
	rc.ticker = time.NewTicker(rc.Interval)
	go func() {
		for {
			select {
			case <-rc.ticker.C:
				rc.generateMetric()
			case <-rc.quitChannel:
				return
			}
		}
	}()
}

// Stop is for stopping itself
func (rc *RequestCounter) Stop() error {
	rc.ticker.Stop()
	close(rc.quitChannel)

	return nil
}

// Increase is for increase message count increase per env
func (rc *RequestCounter) Increase(env string) {
	rc.Lock()
	defer rc.Unlock()
	if value, ok := rc.envCount[env]; ok {
		rc.envCount[env] = value + 1
	} else {
		rc.envCount[env] = 1
	}
}

// translate all count to metrics and push it to chan
func (rc *RequestCounter) generateMetric() {
	rc.Lock()
	defer rc.Unlock()
	timestamp := getCurrentEpochTime()
	isAllUnavailable := true
	for env, count := range rc.envCount {
		if count != 0 {
			isAllUnavailable = false
		}
		prefix := "fjord.burrow." + env + "." + rc.Name
		envTag := "env=" + env
		message := strings.Join([]string{prefix, strconv.Itoa(count), timestamp, rc.Postfix, envTag}, " ")
		rc.ProducerChan <- message
	}
	if isAllUnavailable {
		rc.unavailableCount++
	} else {
		rc.unavailableCount = 0
	}
	rc.envCount = make(map[string]int)
}

func getCurrentEpochTime() string {
	return strconv.FormatInt(time.Now().Unix(), 10)
}

// IsMetricAvailable is for test if Burrow is sending Lag information to Rainbow
func (rc *RequestCounter) IsMetricAvailable() bool {
	rc.Lock()
	defer rc.Unlock()
	// We set healthy threshold is 8 here, i.e. 8 min
	return rc.unavailableCount < 8
}
