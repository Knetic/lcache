lcache
====

`lcache` (L-Cache) is an in-memory read-through generic LRU cache for Go. It is heavily inspired by [Guava's LoadingCache](https://github.com/google/guava/wiki/CachesExplained), which the author has found to be one of the single most useful libraries in production use.

## How to use

To make a cache, implement and provide a `Loader`:
```go
type staticLoader struct {
    data map[string]int
}

func (loader *staticLoader) Load(key string) (interface{}, error) {
    // in practice, this would be some data that is expensive to load
    if num, exists := (*loader.data)[key]; !exists {
        return nil, nil
    } else {
        return num, nil
    }
}

data := map[string]int {"some key": 42}
cache, _ := lcache.NewCache(Params{
    Loader: &staticLoader{data: data},
    ...
})

value, _ := c.Get("some key")

// prints 42
fmt.Println(value)
```

## Specifics

