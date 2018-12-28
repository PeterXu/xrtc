package webrtc

import (
	"sync"

	log "github.com/Sirupsen/logrus"
)

type Cache struct {
	sync.RWMutex
	items map[string]*CacheItem
}

func NewCache() *Cache {
	return &Cache{items: make(map[string]*CacheItem)}
}

type CacheItem struct {
	data    interface{}
	timeout uint32
	ctime   uint32
	utime   uint32
}

func NewCacheItem(data interface{}, timeout uint32) *CacheItem {
	return &CacheItem{data: data, timeout: timeout, ctime: NowMs(), utime: NowMs()}
}

func (h *Cache) Get(key string) *CacheItem {
	h.RLock()
	defer h.RUnlock()
	if i, ok := h.items[key]; ok {
		return i
	}
	return nil
}

func (h *Cache) Set(key string, item *CacheItem) {
	h.Lock()
	defer h.Unlock()
	h.items[key] = item
}

func (h *Cache) Update(key string) bool {
	h.Lock()
	defer h.Unlock()
	if c, ok := h.items[key]; ok {
		c.utime = NowMs()
		return true
	}
	return false
}

func (h *Cache) ClearTimeout() {
	const kMaxTimeout = 600 * 1000    // ms
	const kDefaultTimeout = 30 * 1000 // ms

	nowTime := NowMs()
	var desperated []string
	h.RLock()
	for k, v := range h.items {
		timeout := v.timeout
		if timeout == 0 || timeout > kMaxTimeout {
			timeout = kDefaultTimeout
		}
		if nowTime >= v.utime+timeout {
			desperated = append(desperated, k)
		}
	}
	h.RUnlock()

	if len(desperated) > 0 {
		log.Println("[cache] clear timeout, size=", len(desperated))
		h.Lock()
		for index := range desperated {
			delete(h.items, desperated[index])
		}
		h.Unlock()
	}
}
