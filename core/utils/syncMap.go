package utils

import "sync"

// SyncNestedMap is used for goRainbow cluster-info mapping
// No need to use RWMutex, because only main thread would read
// and two threads would write.
type SyncNestedMap struct {
	sync.Mutex
	infoMap     map[string]map[string]interface{}
	clusterLock map[string]*sync.Mutex
}

func (snm *SyncNestedMap) Init() {
	snm.Lock()
	defer snm.Unlock()

	snm.infoMap = make(map[string]map[string]interface{})
	snm.clusterLock = make(map[string]*sync.Mutex)
}

// SetLock to set a refined lock, on cluster-level,
// to avoid blocking, to improve performance
func (snm *SyncNestedMap) SetLock(cluster string) bool {
	snm.Lock()
	defer snm.Unlock()

	if _, ok := snm.infoMap[cluster]; !ok {
		return false
	}
	snm.clusterLock[cluster].Lock()
	return true
}

func (snm *SyncNestedMap) ReleaseLock(cluster string) bool {
	snm.Lock()
	defer snm.Unlock()

	if _, ok := snm.infoMap[cluster]; !ok {
		return false
	}
	snm.clusterLock[cluster].Unlock()
	return true
}

func (snm *SyncNestedMap) GetChild(cluster string) map[string]interface{} {
	snm.Lock()
	defer snm.Unlock()
	if _, ok := snm.infoMap[cluster]; ok {

	} else {
		snm.infoMap[cluster] = make(map[string]interface{})
		snm.clusterLock[cluster] = &sync.Mutex{}
	}
	return snm.infoMap[cluster]
}

func (snm *SyncNestedMap) DeregisterChild(cluster string, consumer string) {
	// Refined cluster-level lock.
	snm.SetLock(cluster)
	defer snm.ReleaseLock(cluster)

	// May need to add a judge
	delete(snm.infoMap[cluster], consumer)
}
