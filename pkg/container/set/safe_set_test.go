/*
 *     Copyright 2020 The Dragonfly Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package set

import (
	"math/rand"
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

const N = 1000

func TestSafeSetAdd(t *testing.T) {
	tests := []struct {
		name   string
		value  any
		expect func(t *testing.T, ok bool, s SafeSet, value any)
	}{
		{
			name:  "add value",
			value: "foo",
			expect: func(t *testing.T, ok bool, s SafeSet, value any) {
				assert := assert.New(t)
				assert.Equal(ok, true)
				assert.Equal(s.Values(), []any{value})
			},
		},
		{
			name:  "add value failed",
			value: "foo",
			expect: func(t *testing.T, _ bool, s SafeSet, value any) {
				assert := assert.New(t)
				ok := s.Add("foo")
				assert.Equal(ok, false)
				assert.Equal(s.Values(), []any{value})
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := NewSafeSet()
			tc.expect(t, s.Add(tc.value), s, tc.value)
		})
	}
}

func TestSafeSetAdd_Concurrent(t *testing.T) {
	runtime.GOMAXPROCS(2)

	s := NewSafeSet()
	nums := rand.Perm(N)

	var wg sync.WaitGroup
	wg.Add(len(nums))
	for i := 0; i < len(nums); i++ {
		go func(i int) {
			s.Add(i)
			wg.Done()
		}(i)
	}

	wg.Wait()
	for _, n := range nums {
		if !s.Contains(n) {
			t.Errorf("Set is missing element: %v", n)
		}
	}
}

func TestSafeSetDelete(t *testing.T) {
	tests := []struct {
		name   string
		value  any
		expect func(t *testing.T, s SafeSet, value any)
	}{
		{
			name:  "delete value",
			value: "foo",
			expect: func(t *testing.T, s SafeSet, value any) {
				assert := assert.New(t)
				s.Delete(value)
				assert.Equal(s.Len(), uint(0))
			},
		},
		{
			name:  "delete value does not exist",
			value: "foo",
			expect: func(t *testing.T, s SafeSet, _ any) {
				assert := assert.New(t)
				s.Delete("bar")
				assert.Equal(s.Len(), uint(1))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := NewSafeSet()
			s.Add(tc.value)
			tc.expect(t, s, tc.value)
		})
	}
}

func TestSafeSetDelete_Concurrent(t *testing.T) {
	runtime.GOMAXPROCS(2)

	s := NewSafeSet()
	nums := rand.Perm(N)
	for _, v := range nums {
		s.Add(v)
	}

	var wg sync.WaitGroup
	wg.Add(len(nums))
	for _, v := range nums {
		go func(i int) {
			s.Delete(i)
			wg.Done()
		}(v)
	}
	wg.Wait()

	if s.Len() != 0 {
		t.Errorf("Expected len 0; got %v", s.Len())
	}
}

func TestSafeSetContains(t *testing.T) {
	tests := []struct {
		name   string
		value  any
		expect func(t *testing.T, s SafeSet, value any)
	}{
		{
			name:  "contains value",
			value: "foo",
			expect: func(t *testing.T, s SafeSet, value any) {
				assert := assert.New(t)
				assert.Equal(s.Contains(value), true)
			},
		},
		{
			name:  "contains value does not exist",
			value: "foo",
			expect: func(t *testing.T, s SafeSet, _ any) {
				assert := assert.New(t)
				assert.Equal(s.Contains("bar"), false)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := NewSafeSet()
			s.Add(tc.value)
			tc.expect(t, s, tc.value)
		})
	}
}

func TestSafeSetContains_Concurrent(t *testing.T) {
	runtime.GOMAXPROCS(2)

	s := NewSafeSet()
	nums := rand.Perm(N)
	interfaces := make([]any, 0)
	for _, v := range nums {
		s.Add(v)
		interfaces = append(interfaces, v)
	}

	var wg sync.WaitGroup
	for range nums {
		wg.Add(1)
		go func() {
			s.Contains(interfaces...)
			wg.Done()
		}()
	}
	wg.Wait()
}

func TestSetSafeLen(t *testing.T) {
	tests := []struct {
		name   string
		expect func(t *testing.T, s SafeSet)
	}{
		{
			name: "get length",
			expect: func(t *testing.T, s SafeSet) {
				assert := assert.New(t)
				s.Add("foo")
				assert.Equal(s.Len(), uint(1))
			},
		},
		{
			name: "get empty set length",
			expect: func(t *testing.T, s SafeSet) {
				assert := assert.New(t)
				assert.Equal(s.Len(), uint(0))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := NewSafeSet()
			tc.expect(t, s)
		})
	}
}

func TestSafeSetLen_Concurrent(t *testing.T) {
	runtime.GOMAXPROCS(2)

	s := NewSafeSet()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		elems := s.Len()
		for i := 0; i < N; i++ {
			newElems := s.Len()
			if newElems < elems {
				t.Errorf("Len shrunk from %v to %v", elems, newElems)
			}
		}
		wg.Done()
	}()

	for i := 0; i < N; i++ {
		s.Add(rand.Int())
	}
	wg.Wait()
}

func TestSafeSetValues(t *testing.T) {
	tests := []struct {
		name   string
		expect func(t *testing.T, s SafeSet)
	}{
		{
			name: "get values",
			expect: func(t *testing.T, s SafeSet) {
				assert := assert.New(t)
				s.Add("foo")
				assert.Equal(s.Values(), []any{"foo"})
			},
		},
		{
			name: "get empty values",
			expect: func(t *testing.T, s SafeSet) {
				assert := assert.New(t)
				assert.Equal(s.Values(), []any(nil))
			},
		},
		{
			name: "get multi values",
			expect: func(t *testing.T, s SafeSet) {
				assert := assert.New(t)
				s.Add("foo")
				s.Add("bar")
				assert.Contains(s.Values(), "bar")
				assert.Contains(s.Values(), "foo")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := NewSafeSet()
			tc.expect(t, s)
		})
	}
}

func TestSafeSetValues_Concurrent(t *testing.T) {
	runtime.GOMAXPROCS(2)

	s := NewSafeSet()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		elems := s.Values()
		for i := 0; i < N; i++ {
			newElems := s.Values()
			if len(newElems) < len(elems) {
				t.Errorf("Values shrunk from %v to %v", elems, newElems)
			}
		}
		wg.Done()
	}()

	for i := 0; i < N; i++ {
		s.Add(rand.Int())
	}
	wg.Wait()
}

func TestSafeSetRange(t *testing.T) {
	tests := []struct {
		name   string
		expect func(t *testing.T, s SafeSet)
	}{
		{
			name: "range values",
			expect: func(t *testing.T, s SafeSet) {
				assert := assert.New(t)
				s.Add("foo")
				s.Range(func(v any) bool {
					assert.Equal(v, "foo")
					return true
				})
			},
		},
		{
			name: "range values failed",
			expect: func(t *testing.T, s SafeSet) {
				assert := assert.New(t)
				s.Add("foo")
				s.Add("bar")
				s.Range(func(v any) bool {
					assert.Equal(s.Contains(v), true)
					return false
				})
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := NewSafeSet()
			tc.expect(t, s)
		})
	}
}

func TestSafeSetRange_Concurrent(t *testing.T) {
	runtime.GOMAXPROCS(2)

	s := NewSafeSet()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		elems := s.Values()
		i := 0
		s.Range(func(v any) bool {
			i++
			return true
		})
		if i < len(elems) {
			t.Errorf("Values shrunk from %v to %v", elems, i)
		}
		wg.Done()
	}()

	for i := 0; i < N; i++ {
		s.Add(rand.Int())
	}
	wg.Wait()
}

func TestSafeSetClear(t *testing.T) {
	tests := []struct {
		name   string
		expect func(t *testing.T, s SafeSet)
	}{
		{
			name: "clear empty set",
			expect: func(t *testing.T, s SafeSet) {
				assert := assert.New(t)
				s.Clear()
				assert.Equal(s.Values(), []any(nil))
			},
		},
		{
			name: "clear set",
			expect: func(t *testing.T, s SafeSet) {
				assert := assert.New(t)
				assert.Equal(s.Add("foo"), true)
				s.Clear()
				assert.Equal(s.Values(), []any(nil))
				assert.Equal(s.Add("foo"), true)
				assert.Equal(s.Values(), []any{"foo"})
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := NewSafeSet()
			tc.expect(t, s)
		})
	}
}

func TestSafeSetClear_Concurrent(t *testing.T) {
	runtime.GOMAXPROCS(2)

	s := NewSafeSet()
	nums := rand.Perm(N)

	var wg sync.WaitGroup
	wg.Add(len(nums))
	for i := 0; i < len(nums); i++ {
		go func(i int) {
			s.Add(i)
			s.Clear()
			wg.Done()
		}(i)
	}

	wg.Wait()
	for _, n := range nums {
		if s.Contains(n) {
			t.Errorf("SafeSet contains element: %v", n)
		}
	}
}
