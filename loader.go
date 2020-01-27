package lcache

/*
	A Loader is a user-provided type that contains the logic needed to load a given key into the cache.
	Generally, the key will refer to something like a key name in Redis, or a a table+row in Cassandra.
*/
type Loader interface {

	// This will be called by the library at the point when a value needs to be loaded/reloaded from cache.
	Load(key string) interface{}
}