/*
 *     Copyright 2022 The Dragonfly Authors
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

package dag

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDAG(t *testing.T) {
	d := NewDAG()
	assert := assert.New(t)
	assert.Equal(reflect.TypeOf(d).Elem().Name(), "dag")
}

func TestDAGAddVertex(t *testing.T) {
	tests := []struct {
		name   string
		id     string
		value  any
		expect func(t *testing.T, d DAG, err error)
	}{
		{
			name:  "add vertex",
			id:    mockVertexID,
			value: mockVertexValue,
			expect: func(t *testing.T, d DAG, err error) {
				assert := assert.New(t)
				assert.NoError(err)
			},
		},
		{
			name:  "vertex already exists",
			id:    mockVertexID,
			value: mockVertexValue,
			expect: func(t *testing.T, d DAG, err error) {
				assert := assert.New(t)
				assert.NoError(err)

				assert.EqualError(d.AddVertex(mockVertexID, mockVertexValue), ErrVertexAlreadyExists.Error())
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := NewDAG()
			tc.expect(t, d, d.AddVertex(tc.id, tc.name))
		})
	}
}

func TestDAGDeleteVertex(t *testing.T) {
	tests := []struct {
		name   string
		expect func(t *testing.T, d DAG)
	}{
		{
			name: "delete vertex",
			expect: func(t *testing.T, d DAG) {
				assert := assert.New(t)
				if err := d.AddVertex(mockVertexID, mockVertexValue); err != nil {
					assert.NoError(err)
				}
				d.DeleteVertex(mockVertexID)

				_, err := d.GetVertex(mockVertexID)
				assert.EqualError(err, ErrVertexNotFound.Error())
			},
		},
		{
			name: "delete vertex with edges",
			expect: func(t *testing.T, d DAG) {
				assert := assert.New(t)

				var (
					mockToVertexID = "baz"
				)
				if err := d.AddVertex(mockToVertexID, mockVertexValue); err != nil {
					assert.NoError(err)
				}

				if err := d.AddVertex(mockVertexID, mockVertexValue); err != nil {
					assert.NoError(err)
				}

				if err := d.AddEdge(mockVertexID, mockToVertexID); err != nil {
					assert.NoError(err)
				}

				d.DeleteVertex(mockVertexID)

				_, err := d.GetVertex(mockVertexID)
				assert.EqualError(err, ErrVertexNotFound.Error())

				vertex, err := d.GetVertex(mockToVertexID)
				assert.NoError(err)
				assert.Equal(vertex.Parents.Len(), uint(0))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := NewDAG()
			tc.expect(t, d)
		})
	}
}

func TestDAGGetVertex(t *testing.T) {
	tests := []struct {
		name   string
		expect func(t *testing.T, d DAG)
	}{
		{
			name: "get vertex",
			expect: func(t *testing.T, d DAG) {
				assert := assert.New(t)
				if err := d.AddVertex(mockVertexID, mockVertexValue); err != nil {
					assert.NoError(err)
				}

				vertex, err := d.GetVertex(mockVertexID)
				assert.NoError(err)
				assert.Equal(vertex.ID, mockVertexID)
				assert.Equal(vertex.Value, mockVertexValue)
				assert.Equal(vertex.Parents.Len(), uint(0))
				assert.Equal(vertex.Children.Len(), uint(0))
			},
		},
		{
			name: "vertex not found",
			expect: func(t *testing.T, d DAG) {
				assert := assert.New(t)
				_, err := d.GetVertex(mockVertexID)
				assert.EqualError(err, ErrVertexNotFound.Error())
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := NewDAG()
			tc.expect(t, d)
		})
	}
}

func TestDAGVertexLenVertex(t *testing.T) {
	tests := []struct {
		name   string
		expect func(t *testing.T, d DAG)
	}{
		{
			name: "get length of vertex",
			expect: func(t *testing.T, d DAG) {
				assert := assert.New(t)
				if err := d.AddVertex(mockVertexID, mockVertexValue); err != nil {
					assert.NoError(err)
				}

				d.LenVertex()
				assert.Equal(d.LenVertex(), 1)

				d.DeleteVertex(mockVertexID)
				assert.Equal(d.LenVertex(), 0)
			},
		},
		{
			name: "empty dag",
			expect: func(t *testing.T, d DAG) {
				assert := assert.New(t)
				assert.Equal(d.LenVertex(), 0)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := NewDAG()
			tc.expect(t, d)
		})
	}
}

func TestDAGRange(t *testing.T) {
	tests := []struct {
		name   string
		expect func(t *testing.T, d DAG)
	}{
		{
			name: "range vertices",
			expect: func(t *testing.T, d DAG) {
				assert := assert.New(t)
				if err := d.AddVertex(mockVertexID, mockVertexValue); err != nil {
					assert.NoError(err)
				}

				d.RangeVertex(func(k string, v *Vertex) bool {
					assert.Equal(k, mockVertexID)
					return true
				})
			},
		},
		{
			name: "range failed",
			expect: func(t *testing.T, d DAG) {
				assert := assert.New(t)

				var (
					mockVertexEID = "bae"
					mockVertexFID = "baf"
				)
				if err := d.AddVertex(mockVertexEID, mockVertexValue); err != nil {
					assert.NoError(err)
				}

				if err := d.AddVertex(mockVertexFID, mockVertexValue); err != nil {
					assert.NoError(err)
				}

				d.RangeVertex(func(k string, v *Vertex) bool {
					assert.Contains([]string{mockVertexEID, mockVertexFID}, k)
					return false
				})
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := NewDAG()
			tc.expect(t, d)
		})
	}
}

func TestDAGAddEdge(t *testing.T) {
	tests := []struct {
		name   string
		expect func(t *testing.T, d DAG)
	}{
		{
			name: "add edge",
			expect: func(t *testing.T, d DAG) {
				assert := assert.New(t)
				var (
					mockVertexEID = "bae"
					mockVertexFID = "baf"
					mockVertexGID = "bag"
					mockVertexHID = "bah"
					mockVertexIID = "bai"
				)

				if err := d.AddVertex(mockVertexEID, mockVertexValue); err != nil {
					assert.NoError(err)
				}

				if err := d.AddVertex(mockVertexFID, mockVertexValue); err != nil {
					assert.NoError(err)
				}

				if err := d.AddVertex(mockVertexGID, mockVertexValue); err != nil {
					assert.NoError(err)
				}

				if err := d.AddVertex(mockVertexHID, mockVertexValue); err != nil {
					assert.NoError(err)
				}

				if err := d.AddVertex(mockVertexIID, mockVertexValue); err != nil {
					assert.NoError(err)
				}

				if err := d.AddEdge(mockVertexEID, mockVertexFID); err != nil {
					assert.NoError(err)
				}

				if err := d.AddEdge(mockVertexFID, mockVertexGID); err != nil {
					assert.NoError(err)
				}

				if err := d.AddEdge(mockVertexFID, mockVertexHID); err != nil {
					assert.NoError(err)
				}

				if err := d.AddEdge(mockVertexGID, mockVertexIID); err != nil {
					assert.NoError(err)
				}

				if err := d.AddEdge(mockVertexIID, mockVertexHID); err != nil {
					assert.NoError(err)
				}
			},
		},
		{
			name: "cycle between vertices",
			expect: func(t *testing.T, d DAG) {
				assert := assert.New(t)
				var (
					mockVertexEID = "bae"
					mockVertexFID = "baf"
					mockVertexGID = "bag"
					mockVertexHID = "bah"
					mockVertexIID = "bai"
				)

				if err := d.AddVertex(mockVertexEID, mockVertexValue); err != nil {
					assert.NoError(err)
				}

				if err := d.AddVertex(mockVertexFID, mockVertexValue); err != nil {
					assert.NoError(err)
				}

				if err := d.AddVertex(mockVertexGID, mockVertexValue); err != nil {
					assert.NoError(err)
				}

				if err := d.AddVertex(mockVertexHID, mockVertexValue); err != nil {
					assert.NoError(err)
				}

				if err := d.AddVertex(mockVertexIID, mockVertexValue); err != nil {
					assert.NoError(err)
				}

				if err := d.AddEdge(mockVertexEID, mockVertexEID); err != nil {
					assert.EqualError(err, ErrCycleBetweenVertices.Error())
				}

				if err := d.AddEdge(mockVertexEID, mockVertexFID); err != nil {
					assert.NoError(err)
				}

				if err := d.AddEdge(mockVertexFID, mockVertexEID); err != nil {
					assert.EqualError(err, ErrCycleBetweenVertices.Error())
				}

				if err := d.AddEdge(mockVertexFID, mockVertexGID); err != nil {
					assert.NoError(err)
				}

				if err := d.AddEdge(mockVertexGID, mockVertexEID); err != nil {
					assert.EqualError(err, ErrCycleBetweenVertices.Error())
				}

				if err := d.AddEdge(mockVertexGID, mockVertexHID); err != nil {
					assert.NoError(err)
				}

				if err := d.AddEdge(mockVertexHID, mockVertexIID); err != nil {
					assert.NoError(err)
				}

				if err := d.AddEdge(mockVertexIID, mockVertexEID); err != nil {
					assert.EqualError(err, ErrCycleBetweenVertices.Error())
				}
			},
		},
		{
			name: "vertex not found",
			expect: func(t *testing.T, d DAG) {
				assert := assert.New(t)
				var (
					mockVertexEID = "bae"
					mockVertexFID = "baf"
				)

				if err := d.AddVertex(mockVertexEID, mockVertexValue); err != nil {
					assert.NoError(err)
				}

				if err := d.AddEdge(mockVertexEID, mockVertexFID); err != nil {
					assert.EqualError(err, ErrVertexNotFound.Error())
				}

				if err := d.AddEdge(mockVertexFID, mockVertexEID); err != nil {
					assert.EqualError(err, ErrVertexNotFound.Error())
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := NewDAG()
			tc.expect(t, d)
		})
	}
}

func TestDAGDeleteEdge(t *testing.T) {
	tests := []struct {
		name   string
		expect func(t *testing.T, d DAG)
	}{
		{
			name: "delete edge",
			expect: func(t *testing.T, d DAG) {
				assert := assert.New(t)
				var (
					mockVertexEID = "bae"
					mockVertexFID = "baf"
				)

				if err := d.AddVertex(mockVertexEID, mockVertexValue); err != nil {
					assert.NoError(err)
				}

				if err := d.AddVertex(mockVertexFID, mockVertexValue); err != nil {
					assert.NoError(err)
				}

				if err := d.AddEdge(mockVertexEID, mockVertexFID); err != nil {
					assert.NoError(err)
				}

				if err := d.DeleteEdge(mockVertexFID, mockVertexEID); err != nil {
					assert.NoError(err)
				}

				vf, err := d.GetVertex(mockVertexFID)
				assert.NoError(err)
				assert.Equal(vf.Parents.Len(), uint(1))
				ve, err := d.GetVertex(mockVertexEID)
				assert.NoError(err)
				assert.Equal(ve.Children.Len(), uint(1))

				if err := d.DeleteEdge(mockVertexEID, mockVertexFID); err != nil {
					assert.NoError(err)
				}

				vf, err = d.GetVertex(mockVertexFID)
				assert.NoError(err)
				assert.Equal(vf.Parents.Len(), uint(0))
				ve, err = d.GetVertex(mockVertexEID)
				assert.NoError(err)
				assert.Equal(ve.Children.Len(), uint(0))
			},
		},
		{
			name: "vertex not found",
			expect: func(t *testing.T, d DAG) {
				assert := assert.New(t)
				var (
					mockVertexEID = "bae"
					mockVertexFID = "baf"
				)

				if err := d.AddVertex(mockVertexEID, mockVertexValue); err != nil {
					assert.NoError(err)
				}

				if err := d.AddEdge(mockVertexEID, mockVertexFID); err != nil {
					assert.EqualError(err, ErrVertexNotFound.Error())
				}

				if err := d.AddEdge(mockVertexFID, mockVertexEID); err != nil {
					assert.EqualError(err, ErrVertexNotFound.Error())
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := NewDAG()
			tc.expect(t, d)
		})
	}
}

func BenchmarkDAGAddVertex(b *testing.B) {
	var ids []string
	d := NewDAG()
	for n := 0; n < b.N; n++ {
		ids = append(ids, fmt.Sprint(n))
	}

	b.ResetTimer()
	for _, id := range ids {
		if err := d.AddVertex(id, nil); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDAGDeleteVertex(b *testing.B) {
	var ids []string
	d := NewDAG()
	for n := 0; n < b.N; n++ {
		id := fmt.Sprint(n)
		if err := d.AddVertex(id, nil); err != nil {
			b.Fatal(err)
		}

		ids = append(ids, id)
	}

	b.ResetTimer()
	for _, id := range ids {
		d.DeleteVertex(id)
	}
}

func BenchmarkDAGDeleteVertexWithMultiEdges(b *testing.B) {
	var ids []string
	d := NewDAG()
	for n := 0; n < b.N; n++ {
		id := fmt.Sprint(n)
		if err := d.AddVertex(id, nil); err != nil {
			b.Fatal(err)
		}

		ids = append(ids, id)
	}

	edgeCount := 5
	for index, id := range ids {
		if index+edgeCount > len(ids)-1 {
			break
		}

		for n := 1; n < edgeCount; n++ {
			if err := d.AddEdge(id, ids[index+n]); err != nil {
				b.Fatal(err)
			}
		}
	}

	b.ResetTimer()
	for _, id := range ids {
		d.DeleteVertex(id)
	}
}

func BenchmarkDAGAddEdge(b *testing.B) {
	var ids []string
	d := NewDAG()
	for n := 0; n < b.N; n++ {
		id := fmt.Sprint(n)
		if err := d.AddVertex(id, nil); err != nil {
			b.Fatal(err)
		}

		ids = append(ids, id)
	}

	b.ResetTimer()
	for index, id := range ids {
		if index < 1 {
			continue
		}

		if err := d.AddEdge(id, ids[index-1]); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDAGAddEdgeWithMultiEdges(b *testing.B) {
	var ids []string
	d := NewDAG()
	for n := 0; n < b.N; n++ {
		id := fmt.Sprint(n)
		if err := d.AddVertex(id, nil); err != nil {
			b.Fatal(err)
		}

		ids = append(ids, id)
	}

	edgeCount := 5
	for index, id := range ids {
		if index+edgeCount > len(ids)-1 {
			break
		}

		for n := 1; n < edgeCount; n++ {
			if err := d.AddEdge(id, ids[index+n]); err != nil {
				b.Fatal(err)
			}
		}
	}

	b.ResetTimer()
	for index, id := range ids {
		if index+edgeCount+1 > len(ids)-1 {
			break
		}

		if err := d.AddEdge(id, ids[index+edgeCount+1]); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDAGDeleteEdge(b *testing.B) {
	var ids []string
	d := NewDAG()
	for n := 0; n < b.N; n++ {
		id := fmt.Sprint(n)
		if err := d.AddVertex(id, nil); err != nil {
			b.Fatal(err)
		}

		ids = append(ids, id)
	}

	for index, id := range ids {
		if index < 1 {
			continue
		}

		if err := d.AddEdge(id, ids[index-1]); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for index, id := range ids {
		if index < 1 {
			continue
		}

		if err := d.DeleteEdge(id, ids[index-1]); err != nil {
			b.Fatal(err)
		}
	}
}
