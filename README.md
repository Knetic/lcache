lcache
====

`lcache` (L-Cache) is an in-memory read-through generic LRU cache for Go. It is heavily inspired by [Guava's LoadingCache](https://github.com/google/guava/wiki/CachesExplained), which the author has found to be one of the single most useful libraries in production use.

## How to use

To make a cache, 

```go

type ConcatLoader struct {
	ConcatWith string
}

func (this ConcatLoader) Load(key string) (interface{}, error) {
	return key + this.ConcatWith, nil
}

var c := lcache.Cache {
	Loader: ConcatLoader{ConcatWith: " is the key!"},
}

value, _ := c.Get("something")

// prints "something is the key!"
fmt.Println(value)
```

## Specifics

