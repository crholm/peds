package peds

import "fmt"

const shiftSize = 5
const nodeSize = 32
const shiftBitMask = 0x1F

func uintMin(a, b uint) uint {
	if a < b {
		return a
	}

	return b
}

// ////////////
// / Vector ///
// ////////////
type commonNode interface{}

var emptyCommonNode commonNode = []commonNode{}

// A Vector is an ordered persistent/immutable collection of items corresponding roughly
// to the use cases for a slice.
type Vector[T any] struct {
	tail  []T
	root  commonNode
	len   uint
	shift uint
}

// NewVector returns a new vector containing the items provided in items.
func NewVector[T any](items ...T) *Vector[T] {
	// TODO: Could potentially do something smarter with a factory for a certain type
	//       if this results in a lot of allocations.
	tail := make([]T, 0)
	v := &Vector[T]{root: emptyCommonNode, shift: shiftSize, tail: tail}
	return v.Append(items...)
}

// Append returns a new vector with item(s) appended to it.
func (v *Vector[T]) Append(item ...T) *Vector[T] {
	result := v
	itemLen := uint(len(item))
	for insertOffset := uint(0); insertOffset < itemLen; {
		tailLen := result.len - result.tailOffset()
		tailFree := nodeSize - tailLen
		if tailFree == 0 {
			result = result.pushLeafNode(result.tail)
			result.tail = make([]T, 0)
			tailFree = nodeSize
			tailLen = 0
		}

		batchLen := uintMin(itemLen-insertOffset, tailFree)
		newTail := make([]T, 0, tailLen+batchLen)
		newTail = append(newTail, result.tail...)
		newTail = append(newTail, item[insertOffset:insertOffset+batchLen]...)
		result = &Vector[T]{root: result.root, tail: newTail, len: result.len + batchLen, shift: result.shift}
		insertOffset += batchLen
	}

	return result
}

func (v *Vector[T]) tailOffset() uint {
	if v.len < nodeSize {
		return 0
	}

	return ((v.len - 1) >> shiftSize) << shiftSize
}

func (v *Vector[T]) pushLeafNode(node []T) *Vector[T] {
	var newRoot commonNode
	newShift := v.shift

	// Root overflow?
	if (v.len >> shiftSize) > (1 << v.shift) {
		newNode := newPath(v.shift, node)
		newRoot = commonNode([]commonNode{v.root, newNode})
		newShift = v.shift + shiftSize
	} else {
		newRoot = v.pushTail(v.shift, v.root, node)
	}

	return &Vector[T]{root: newRoot, tail: v.tail, len: v.len, shift: newShift}
}

func newPath(shift uint, node commonNode) commonNode {
	if shift == 0 {
		return node
	}

	return newPath(shift-shiftSize, commonNode([]commonNode{node}))
}

func (v *Vector[T]) pushTail(level uint, parent commonNode, tailNode []T) commonNode {
	subIdx := ((v.len - 1) >> level) & shiftBitMask
	parentNode := parent.([]commonNode)
	ret := make([]commonNode, subIdx+1)
	copy(ret, parentNode)
	var nodeToInsert commonNode

	if level == shiftSize {
		nodeToInsert = tailNode
	} else if subIdx < uint(len(parentNode)) {
		nodeToInsert = v.pushTail(level-shiftSize, parentNode[subIdx], tailNode)
	} else {
		nodeToInsert = newPath(level-shiftSize, tailNode)
	}

	ret[subIdx] = nodeToInsert
	return ret
}

// Len returns the length of v.
func (v *Vector[T]) Len() int {
	return int(v.len)
}

// Get returns the element at position i.
func (v *Vector[T]) Get(i int) T {
	if i < 0 || uint(i) >= v.len {
		panic("Index out of bounds")
	}

	return v.sliceFor(uint(i))[i&shiftBitMask]
}

func (v *Vector[T]) sliceFor(i uint) []T {
	if i >= v.tailOffset() {
		return v.tail
	}

	node := v.root
	for level := v.shift; level > 0; level -= shiftSize {
		node = node.([]commonNode)[(i>>level)&shiftBitMask]
	}

	// TODO: Change the nodes of this type to be 32 element arrays of T rather than
	//       slices to get rid of some overhead?
	return node.([]T)
}

// Set returns a new vector with the element at position i set to item.
func (v *Vector[T]) Set(i int, item T) *Vector[T] {
	if i < 0 || uint(i) >= v.len {
		panic("Index out of bounds")
	}

	if uint(i) >= v.tailOffset() {
		newTail := make([]T, len(v.tail))
		copy(newTail, v.tail)
		newTail[i&shiftBitMask] = item
		return &Vector[T]{root: v.root, tail: newTail, len: v.len, shift: v.shift}
	}

	return &Vector[T]{root: v.doAssoc(v.shift, v.root, uint(i), item), tail: v.tail, len: v.len, shift: v.shift}
}

func (v *Vector[T]) doAssoc(level uint, node commonNode, i uint, item T) commonNode {
	if level == 0 {
		ret := make([]T, nodeSize)
		copy(ret, node.([]T))
		ret[i&shiftBitMask] = item
		return ret
	}

	ret := make([]commonNode, nodeSize)
	copy(ret, node.([]commonNode))
	subidx := (i >> level) & shiftBitMask
	ret[subidx] = v.doAssoc(level-shiftSize, ret[subidx], i, item)
	return ret
}

// Range calls f repeatedly passing it each element in v in order as argument until either
// all elements have been visited or f returns false.
func (v *Vector[T]) Range(f func(T) bool) {
	var currentNode []T
	for i := uint(0); i < v.len; i++ {
		if i&shiftBitMask == 0 {
			currentNode = v.sliceFor(i)
		}

		if !f(currentNode[i&shiftBitMask]) {
			return
		}
	}
}

// Slice returns a VectorSlice that refers to all elements [start,stop) in v.
func (v *Vector[T]) Slice(start, stop int) *VectorSlice[T] {
	assertSliceOk(start, stop, v.Len())
	return &VectorSlice[T]{vector: v, start: start, stop: stop}
}

// ToNativeSlice returns a Go slice containing all elements of v
func (v *Vector[T]) ToNativeSlice() []T {
	result := make([]T, 0, v.len)
	for i := uint(0); i < v.len; i += nodeSize {
		result = append(result, v.sliceFor(i)...)
	}

	return result
}

////////////////
//// Slice /////
////////////////

func assertSliceOk(start, stop, len int) {
	if start < 0 {
		panic(fmt.Sprintf("Invalid slice index %d (index must be non-negative)", start))
	}

	if start > stop {
		panic(fmt.Sprintf("Invalid slice index: %d > %d", start, stop))
	}

	if stop > len {
		panic(fmt.Sprintf("Slice bounds out of range, start=%d, stop=%d, len=%d", start, stop, len))
	}
}

// VectorSlice is a slice type backed by a Vector.
type VectorSlice[T any] struct {
	vector      *Vector[T]
	start, stop int
}

// NewVectorSlice returns a new NewVectorSlice containing the items provided in items.
func NewVectorSlice[T any](items ...T) *VectorSlice[T] {
	return &VectorSlice[T]{vector: NewVector[T](items...), start: 0, stop: len(items)}
}

// Len returns the length of s.
func (s *VectorSlice[T]) Len() int {
	return s.stop - s.start
}

// Get returns the element at position i.
func (s *VectorSlice[T]) Get(i int) T {
	if i < 0 || s.start+i >= s.stop {
		panic("Index out of bounds")
	}

	return s.vector.Get(s.start + i)
}

// Set returns a new slice with the element at position i set to item.
func (s *VectorSlice[T]) Set(i int, item T) *VectorSlice[T] {
	if i < 0 || s.start+i >= s.stop {
		panic("Index out of bounds")
	}

	return s.vector.Set(s.start+i, item).Slice(s.start, s.stop)
}

// Append returns a new slice with item(s) appended to it.
func (s *VectorSlice[T]) Append(items ...T) *VectorSlice[T] {
	newSlice := VectorSlice[T]{vector: s.vector, start: s.start, stop: s.stop + len(items)}

	// If this is v slice that has an upper bound that is lower than the backing
	// vector then set the values in the backing vector to achieve some structural
	// sharing.
	itemPos := 0
	for ; s.stop+itemPos < s.vector.Len() && itemPos < len(items); itemPos++ {
		newSlice.vector = newSlice.vector.Set(s.stop+itemPos, items[itemPos])
	}

	// For the rest just append it to the underlying vector
	newSlice.vector = newSlice.vector.Append(items[itemPos:]...)
	return &newSlice
}

// Slice returns a VectorSlice that refers to all elements [start,stop) in s.
func (s *VectorSlice[T]) Slice(start, stop int) *VectorSlice[T] {
	assertSliceOk(start, stop, s.stop-s.start)
	return &VectorSlice[T]{vector: s.vector, start: s.start + start, stop: s.start + stop}
}

// Range calls f repeatedly passing it each element in s in order as argument until either
// all elements have been visited or f returns false.
func (s *VectorSlice[T]) Range(f func(T) bool) {
	var currentNode []T
	for i := uint(s.start); i < uint(s.stop); i++ {
		if i&shiftBitMask == 0 || i == uint(s.start) {
			currentNode = s.vector.sliceFor(uint(i))
		}

		if !f(currentNode[i&shiftBitMask]) {
			return
		}
	}
}
