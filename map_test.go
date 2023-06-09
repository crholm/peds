package peds

import (
	"fmt"
	"testing"
)

func TestOverload(t *testing.T) {
	m := NewMap[string, int]()

	m.Store("a", 0)
	assertEqual(t, 0, m.Len())
	v, ok := m.Load("a")
	assertEqualBool(t, false, ok)

	m = m.Store("a", 1)
	assertEqual(t, 1, m.Len())

	m = m.
		Store("a", 1).
		Store("b", 2).
		Store("c", 3).
		Store("d", 4).
		Store("e", 5).
		Store("f", 6).
		Store("g", 7).
		Store("h", 8)

	m2 := m.Store("a", 11)
	m2 = m2.Store("i", 99).Delete("b").Delete("c")
	assertEqual(t, 8, m.Len())
	assertEqual(t, 7, m2.Len())

	v, ok = m.Load("a")
	assertEqualBool(t, true, ok)
	assertEqual(t, 1, v)

	v, ok = m2.Load("a")
	assertEqualBool(t, true, ok)
	assertEqual(t, 11, v)

}

func TestLenOfNewMap(t *testing.T) {
	m := NewMap[string, int]()
	assertEqual(t, 0, m.Len())

	m2 := NewMap[string, int](MapItem[string, int]{Key: "a", Value: 1})
	assertEqual(t, 1, m2.Len())

	m3 := NewMap[string, int](MapItem[string, int]{Key: "a", Value: 1}, MapItem[string, int]{Key: "b", Value: 2})
	assertEqual(t, 2, m3.Len())
}

func TestLoadAndStore(t *testing.T) {
	m := NewMap[string, int]()

	m2 := m.Store("a", 1)
	assertEqual(t, 0, m.Len())
	assertEqual(t, 1, m2.Len())

	v, ok := m.Load("a")
	assertEqual(t, 0, v)
	assertEqualBool(t, false, ok)

	v, ok = m2.Load("a")
	assertEqual(t, 1, v)
	assertEqualBool(t, true, ok)
}

func TestLoadAndStoreIntKey(t *testing.T) {
	m := NewMap[int, string]()

	m2 := m.Store(1, "")
	v, _ := m.Load(2)
	assertEqualString(t, "", v)

	v, _ = m2.Load(1)
	assertEqualString(t, "", v)
}

func TestLoadAndDeleteExistingItem(t *testing.T) {
	m := NewMap[string, int]()
	m2 := m.Store("a", 1)
	m3 := m.Delete("a")

	assertEqual(t, 0, m3.Len())
	assertEqual(t, 1, m2.Len())

	v, ok := m2.Load("a")
	assertEqualBool(t, true, ok)
	assertEqual(t, 1, v)

	v, ok = m3.Load("a")
	assertEqualBool(t, false, ok)
	assertEqual(t, 0, v)
}

func TestLoadAndDeleteNonExistingItem(t *testing.T) {
	m := NewMap[string, int]()
	m2 := m.Store("a", 1)
	m3 := m2.Delete("b")

	assertEqual(t, 1, m3.Len())
	assertEqual(t, 1, m2.Len())

	v, ok := m2.Load("a")
	assertEqualBool(t, true, ok)
	assertEqual(t, 1, v)

	if m2 != m3 {
		t.Errorf("m2 and m3 are not the same object: %p != %p", m2, m3)
	}
}

func TestRangeAllItems(t *testing.T) {
	m := NewMap[string, int](MapItem[string, int]{Key: "a", Value: 1},
		MapItem[string, int]{Key: "b", Value: 2},
		MapItem[string, int]{Key: "c", Value: 3})
	sum := 0
	m.Range(func(key string, value int) bool {
		sum += value
		return true
	})
	assertEqual(t, 6, sum)
}

func TestRangeStopOnKey(t *testing.T) {
	m := NewMap[string, int](
		MapItem[string, int]{Key: "a", Value: 1},
		MapItem[string, int]{Key: "b", Value: 2},
		MapItem[string, int]{Key: "c", Value: 3},
	)
	count := 0
	m.Range(func(key string, value int) bool {
		if key == "c" || key == "b" {
			return false
		}

		count++
		return true
	})

	if count > 1 {
		t.Errorf("Did not expect count to be more than 1")
	}
}

func TestLargeInsertLookupDelete(t *testing.T) {
	// Is 50000 in original test but that seems crazy slow.
	// More vector allocations, worse generic hash function, any other culprits?
	size := 500
	m := NewMap[string, int]()
	for j := 0; j < size; j++ {
		m = m.Store(fmt.Sprintf("%d", j), j)
	}

	for j := 0; j < size; j++ {
		v, ok := m.Load(fmt.Sprintf("%d", j))
		assertEqualBool(t, true, ok)
		assertEqual(t, v, j)
	}

	for j := 0; j < size; j++ {
		key := fmt.Sprintf("%d", j)
		m = m.Delete(key)
		assertEqual(t, size-j-1, m.Len())
		_, ok := m.Load(key)
		assertEqualBool(t, false, ok)
	}
}

func TestFromToNativeMap(t *testing.T) {
	input := map[string]int{
		"a": 1,
		"b": 2,
		"c": 3}
	m := NewMapFromNativeMap[string, int](input)
	output := m.ToNativeMap()
	assertEqual(t, len(input), len(output))
	for key, value := range input {
		assertEqual(t, value, output[key])
	}
}
