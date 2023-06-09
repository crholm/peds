package peds

import (
	"math"
)

const upperMapLoadFactor float64 = 8.0
const lowerMapLoadFactor float64 = 2.0
const initialMapLoadFactor float64 = (upperMapLoadFactor + lowerMapLoadFactor) / 2

type MapItem[K comparable, V any] struct {
	Key   K
	Value V
}

type privateItemBucket[K comparable, V any] []MapItem[K, V]

// Helper type used during map creation and reallocation
type privateItemBuckets[K comparable, V any] struct {
	buckets []privateItemBucket[K, V]
	length  int
}

func newPrivateItemBuckets[K comparable, V any](itemCount int) *privateItemBuckets[K, V] {
	size := int(float64(itemCount)/initialMapLoadFactor) + 1

	// TODO: The need for parenthesis below are slightly surprising
	buckets := make([](privateItemBucket[K, V]), size)
	return &privateItemBuckets[K, V]{buckets: buckets}
}

type Map[K comparable, V any] struct {
	backingVector *Vector[privateItemBucket[K, V]]
	len           int
}

func (b *privateItemBuckets[K, V]) AddItem(item MapItem[K, V]) {
	ix := int(uint64(genericHash(item.Key)) % uint64(len(b.buckets)))
	bucket := b.buckets[ix]
	if bucket != nil {
		// Hash collision, merge with existing bucket
		for keyIx, bItem := range bucket {
			if item.Key == bItem.Key {
				bucket[keyIx] = item
				return
			}
		}

		b.buckets[ix] = append(bucket, MapItem[K, V]{Key: item.Key, Value: item.Value})
		b.length++
	} else {
		bucket := make(privateItemBucket[K, V], 0, int(math.Max(initialMapLoadFactor, 1.0)))
		b.buckets[ix] = append(bucket, item)
		b.length++
	}
}

func (b *privateItemBuckets[K, V]) AddItemsFromMap(m *Map[K, V]) {
	m.backingVector.Range(func(bucket privateItemBucket[K, V]) bool {
		for _, item := range bucket {
			b.AddItem(item)
		}
		return true
	})
}

func newMap[K comparable, V any](items []MapItem[K, V]) *Map[K, V] {
	buckets := newPrivateItemBuckets[K, V](len(items))
	for _, item := range items {
		buckets.AddItem(item)
	}
	return &Map[K, V]{backingVector: NewVector(buckets.buckets...), len: buckets.length}
}

// NewMap returns a new map containing all items in items.
func NewMap[K comparable, V any](items ...MapItem[K, V]) *Map[K, V] {
	return newMap(items)
}

// NewMapFromNativeMap returns a new Map containing all items in m.
func NewMapFromNativeMap[K comparable, V any](m map[K]V) *Map[K, V] {
	buckets := newPrivateItemBuckets[K, V](len(m))
	for key, value := range m {
		buckets.AddItem(MapItem[K, V]{Key: key, Value: value})
	}

	return &Map[K, V]{backingVector: NewVector(buckets.buckets...), len: buckets.length}
}

// Len returns the number of items in m.
func (m *Map[K, V]) Len() int {
	return int(m.len)
}

func (m *Map[K, V]) pos(key K) int {
	return int(uint64(genericHash(key)) % uint64(m.backingVector.Len()))
}

// Load returns value identified by key. ok is set to true if key exists in the map, false otherwise.
func (m *Map[K, V]) Load(key K) (value V, ok bool) {
	bucket := m.backingVector.Get(m.pos(key))
	if bucket != nil {
		for _, item := range bucket {
			if item.Key == key {
				return item.Value, true
			}
		}
	}

	var zeroValue V
	return zeroValue, false
}

// Store returns a new Map[K, V] containing value identified by key.
func (m *Map[K, V]) Store(key K, value V) *Map[K, V] {
	// Grow backing vector if load factor is too high
	if m.Len() >= m.backingVector.Len()*int(upperMapLoadFactor) {
		buckets := newPrivateItemBuckets[K, V](m.Len() + 1)
		buckets.AddItemsFromMap(m)
		buckets.AddItem(MapItem[K, V]{Key: key, Value: value})
		return &Map[K, V]{backingVector: NewVector[privateItemBucket[K, V]](buckets.buckets...), len: buckets.length}
	}

	pos := m.pos(key)
	bucket := m.backingVector.Get(pos)
	if bucket != nil {
		for ix, item := range bucket {
			if item.Key == key {
				// Overwrite existing item
				newBucket := make(privateItemBucket[K, V], len(bucket))
				copy(newBucket, bucket)
				newBucket[ix] = MapItem[K, V]{Key: key, Value: value}
				return &Map[K, V]{backingVector: m.backingVector.Set(pos, newBucket), len: m.len}
			}
		}

		// Add new item to bucket
		newBucket := make(privateItemBucket[K, V], len(bucket), len(bucket)+1)
		copy(newBucket, bucket)
		newBucket = append(newBucket, MapItem[K, V]{Key: key, Value: value})
		return &Map[K, V]{backingVector: m.backingVector.Set(pos, newBucket), len: m.len + 1}
	}

	item := MapItem[K, V]{Key: key, Value: value}
	newBucket := privateItemBucket[K, V]{item}
	return &Map[K, V]{backingVector: m.backingVector.Set(pos, newBucket), len: m.len + 1}
}

// Delete returns a new Map[K, V] without the element identified by key.
func (m *Map[K, V]) Delete(key K) *Map[K, V] {
	pos := m.pos(key)
	bucket := m.backingVector.Get(pos)
	if bucket != nil {
		newBucket := make(privateItemBucket[K, V], 0)
		for _, item := range bucket {
			if item.Key != key {
				newBucket = append(newBucket, item)
			}
		}

		removedItemCount := len(bucket) - len(newBucket)
		if removedItemCount == 0 {
			return m
		}

		if len(newBucket) == 0 {
			newBucket = nil
		}

		newMap := &Map[K, V]{backingVector: m.backingVector.Set(pos, newBucket), len: m.len - removedItemCount}
		if newMap.backingVector.Len() > 1 && newMap.Len() < newMap.backingVector.Len()*int(lowerMapLoadFactor) {
			// Shrink backing vector if needed to avoid occupying excessive space
			buckets := newPrivateItemBuckets[K, V](newMap.Len())
			buckets.AddItemsFromMap(newMap)
			return &Map[K, V]{backingVector: NewVector(buckets.buckets...), len: buckets.length}
		}

		return newMap
	}

	return m
}

// Range calls f repeatedly passing it each key and value as argument until either
// all elements have been visited or f returns false.
func (m *Map[K, V]) Range(f func(K, V) bool) {
	m.backingVector.Range(func(bucket privateItemBucket[K, V]) bool {
		for _, item := range bucket {
			if !f(item.Key, item.Value) {
				return false
			}
		}
		return true
	})
}

// ToNativeMap returns a native Go map containing all elements of m.
func (m *Map[K, V]) ToNativeMap() map[K]V {
	result := make(map[K]V)
	m.Range(func(key K, value V) bool {
		result[key] = value
		return true
	})

	return result
}
