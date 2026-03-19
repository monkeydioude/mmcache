# mmcache

`mmcache` is a simple, thread-safe, generic in-memory cache for Go 1.23+. It leverages `golang.org/x/sync/singleflight` to prevent the "cache stampede" (or thundering herd) problem, ensuring that expensive value generator functions are only executed once even when multiple goroutines try to retrieve the same missing or expired key concurrently.

## Installation

```sh
go get github.com/monkeydioude/mmcache
```

## Usage

```go
package main

import (
	"fmt"
	"time"

	"github.com/monkeydioude/mmcache"
)

// Example struct to be cached
type User struct {
	ID   int
	Name string
}

func main() {
	// Initialize a new cache with string keys and User values, expiring in 5 minutes
	cache := mmcache.New[string, User](5 * time.Minute)

	// Function to generate the value if it's missing or expired
	fetchUser := func() (User, error) {
		// Simulate an expensive database call or API request
		fmt.Println("Fetching user from database...")
		time.Sleep(1 * time.Second)
		return User{ID: 1, Name: "John Doe"}, nil
	}

	// Gset (Get or Set) retrieves the key "user_1". 
	// If it doesn't exist or is expired, it runs fetchUser().
	user, err := cache.Gset("user_1", fetchUser)
	if err != nil {
		panic(err)
	}
	fmt.Printf("User: %+v\n", user)

	// Subsequent calls within 5 minutes will return the cached value immediately, 
	// without executing fetchUser() again.
	userCached, _ := cache.Gset("user_1", fetchUser)
	fmt.Printf("Cached User: %+v\n", userCached)
}
```

## API Reference

### `New[KeyT comparable, ValueT any](duration time.Duration) *Cache[KeyT, ValueT]`

Creates and returns a new generic cache instance with the specified default expiration duration for all items.

### `Gset(key KeyT, maker func() (ValueT, error)) (ValueT, error)`

`Gset` (short for `Get or Set`) will try to fetch a value from the cache. It will generate the value by running `maker()` and cache it for the appropriate key if:
- The cache for this key does not exist.
- The cache for this key is expired.

`singleflight.Group.Do()` ensures that multiple concurrent calls to `Gset` for the same key will only execute the `maker()` once.

### `Delete(keys ...KeyT)`

Deletes a specific set of keys from the cache.

### `Purge()`

Resets all cache data, effectively emptying the cache.

## Testing

To run the tests:

```sh
make test
```
