package main

import (
	"sync"
)

// thread-safe ~bool~ type
type safeBool struct {
	mu    sync.Mutex
	value bool
}

func (v *safeBool) Set(newValue bool) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.value = newValue
}

func (v *safeBool) Get() bool {
	v.mu.Lock()
	defer v.mu.Unlock()

	return v.value
}

func (v *safeBool) SetIf(newValue bool, oldValue bool) bool {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.value == oldValue {
		v.value = newValue

		return true
	}

	return false
}

/* thread-safe ~int~ type
type safeInt struct {
	mu    sync.Mutex
	value int
}

func (v *safeInt) Set(newValue int) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.value = newValue
}

func (v *safeInt) Get() int {
	v.mu.Lock()
	defer v.mu.Unlock()

	return v.value
}

func (v *safeInt) Add(diff int) int {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.value += diff

	return v.value
}

func (v *safeInt) SetIf(newValue int, oldValue int) bool {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.value == oldValue {
		v.value = newValue

		return true
	}

	return false
}
*/
