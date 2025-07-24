package mmcache

import (
	"fmt"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

type CachedItem[ValueT any] struct {
	value     ValueT
	expiresAt time.Time
}

// Cache takes 2 generics:
// - the type of the key, used to record the cached value
// - the type of the cached value
// In case we need to store values of different type we can still
// use ValueT as `any` (`Cache[string, any]`)
type Cache[KeyT comparable, ValueT any] struct {
	data     map[KeyT]CachedItem[ValueT]
	duration time.Duration
	mutex    sync.Mutex
	sfGroup  singleflight.Group
}

func New[KeyT comparable, ValueT any](duration time.Duration) *Cache[KeyT, ValueT] {
	return &Cache[KeyT, ValueT]{
		data:     make(map[KeyT]CachedItem[ValueT]),
		duration: duration,
	}
}

func (c *Cache[KeyT, ValueT]) singleflightDo(key KeyT, maker func() (ValueT, error)) func() (any, error) {
	return func() (any, error) {
		c.mutex.Lock()
		cache, ok := c.data[key]
		if ok && cache.expiresAt.After(time.Now()) {
			c.mutex.Unlock()
			return cache.value, nil
		}
		// We don't know how much time "maker()" can take.
		// Better release the lock.
		c.mutex.Unlock()
		value, err := maker()
		if err != nil {
			var empty ValueT
			return empty, fmt.Errorf("call to cache maker: %w", err)
		}
		expires := time.Now().Add(c.duration)
		c.mutex.Lock()
		c.data[key] = CachedItem[ValueT]{
			value:     value,
			expiresAt: expires,
		}
		c.mutex.Unlock()
		return value, nil
	}
}

// Gset (short for `Get or Set`) will try to fetch a value from the cache.
// Will generate the value and cache it for the approriate key if:
// - cache for this key does not exist
// - cache is expired
// And will then return the freshly cached value.
func (c *Cache[KeyT, ValueT]) Gset(key KeyT, maker func() (ValueT, error)) (ValueT, error) {
	// singleflight.Group.Do() ensures that multiple concurrent call to this chunk of code
	// will only happen once.
	cachedValue, err, _ := c.sfGroup.Do(fmt.Sprint(key), c.singleflightDo(key, maker))
	if err != nil {
		var empty ValueT
		return empty, err
	}
	return cachedValue.(ValueT), err
}

// Delete deletes a specific set of keys from the cache
func (c *Cache[KeyT, ValueT]) Delete(keys ...KeyT) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	for _, key := range keys {
		delete(c.data, key)
	}
}

// Purge reset the cache data
func (c *Cache[KeyT, ValueT]) Purge() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.data = make(map[KeyT]CachedItem[ValueT])
}
