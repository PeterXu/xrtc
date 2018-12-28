package webrtc

import (
	"sync"
	"time"

	log "github.com/PeterXu/xrtc/util"
)

type Cache struct {
	TAG      string
	items    map[string]*CacheItem
	exitTick chan bool

	sync.RWMutex
}

func NewCache() *Cache {
	c := &Cache{
		TAG:      "[CACHE]",
		items:    make(map[string]*CacheItem),
		exitTick: make(chan bool),
	}
	go c.Run()
	return c
}

// default 30s
const kDefaultCacheTimeout = 30 * 1000 // ms

type CacheItem struct {
	data    interface{} //
	timeout int         // default(30s) if 0
	objtime *ObjTime
}

func NewCacheItem(data interface{}) *CacheItem {
	return NewCacheItemEx(data, 0)
}

func NewCacheItemEx(data interface{}, timeout int) *CacheItem {
	if timeout <= 0 {
		timeout = kDefaultCacheTimeout
	}
	return &CacheItem{
		data:    data,
		timeout: timeout,
		objtime: NewObjTime(),
	}
}

func (h *Cache) Get(key string) *CacheItem {
	h.RLock()
	defer h.RUnlock()
	if i, ok := h.items[key]; ok {
		i.objtime.update()
		return i
	} else {
		return nil
	}
}

func (h *Cache) Set(key string, item *CacheItem) {
	h.Lock()
	defer h.Unlock()
	item.objtime.update()
	h.items[key] = item
}

func (h *Cache) Update(key string) bool {
	h.Lock()
	defer h.Unlock()
	if i, ok := h.items[key]; ok {
		i.objtime.update()
		return true
	} else {
		return false
	}
}

func (h *Cache) ClearTimeout() {
	var desperated []string

	h.RLock()
	for k, v := range h.items {
		if v.objtime.checkTimeout(v.timeout) {
			desperated = append(desperated, k)
		}
	}
	h.RUnlock()

	if len(desperated) > 0 {
		log.Println(h.TAG, "clear timeout, size=", len(desperated))
		h.Lock()
		for index := range desperated {
			delete(h.items, desperated[index])
		}
		h.Unlock()
	}
}

func (h *Cache) Close() {
	h.exitTick <- true
}

func (h *Cache) Run() {
	tickChan := time.NewTicker(time.Second * 30).C
	for {
		select {
		case <-h.exitTick:
			close(h.exitTick)
			log.Println(h.TAG, "Run exit...")
			return
		case <-tickChan:
			h.ClearTimeout()
		}
	}
}
