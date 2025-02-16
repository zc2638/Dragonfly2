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

//go:generate mockgen -destination mocks/dag_mock.go -source dag.go -package mocks

package dag

import (
	"errors"
	"sync"
)

var (
	// ErrVertexNotFound represents vertex not found.
	ErrVertexNotFound = errors.New("vertex not found")

	// ErrVertexAlreadyExists represents vertex already exists.
	ErrVertexAlreadyExists = errors.New("vertex already exists")

	// ErrParnetAlreadyExists represents parent of vertex already exists.
	ErrParnetAlreadyExists = errors.New("parent of vertex already exists")

	// ErrChildAlreadyExists represents child of vertex already exists.
	ErrChildAlreadyExists = errors.New("child of vertex already exists")

	// ErrCycleBetweenVertices represents cycle between vertices.
	ErrCycleBetweenVertices = errors.New("cycle between vertices")
)

// DAG is the interface used for directed acyclic graph.
type DAG interface {
	// AddVertex adds vertex to graph.
	AddVertex(id string, value any) error

	// DeleteVertex deletes vertex graph.
	DeleteVertex(id string)

	// GetVertex gets vertex from graph.
	GetVertex(id string) (*Vertex, error)

	// LenVertex returns length of vertices.
	LenVertex() int

	// RangeVertex calls f sequentially for each key and value present in the vertices.
	// If f returns false, range stops the iteration.
	RangeVertex(fn func(key string, value *Vertex) bool)

	// AddEdge adds edge between two vertices.
	AddEdge(fromVertexID, toVertexID string) error

	// DeleteEdge deletes edge between two vertices.
	DeleteEdge(fromVertexID, toVertexID string) error
}

// dag provides directed acyclic graph function.
type dag struct {
	mu       sync.RWMutex
	vertices map[string]*Vertex
}

// New returns a new DAG interface.
func NewDAG() DAG {
	return &dag{
		vertices: make(map[string]*Vertex),
	}
}

// AddVertex adds vertex to graph.
func (d *dag) AddVertex(id string, value any) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, ok := d.vertices[id]; ok {
		return ErrVertexAlreadyExists
	}

	d.vertices[id] = NewVertex(id, value)
	return nil
}

// DeleteVertex deletes vertex graph.
func (d *dag) DeleteVertex(id string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	vertex, ok := d.vertices[id]
	if !ok {
		return
	}

	vertex.Parents.Range(func(item any) bool {
		parent, ok := item.(*Vertex)
		if !ok {
			return true
		}

		parent.Children.Delete(vertex)
		return true
	})

	vertex.Children.Range(func(item any) bool {
		child, ok := item.(*Vertex)
		if !ok {
			return true
		}

		child.Parents.Delete(vertex)
		return true
	})

	delete(d.vertices, id)
}

// GetVertex gets vertex from graph.
func (d *dag) GetVertex(id string) (*Vertex, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	vertex, ok := d.vertices[id]
	if !ok {
		return nil, ErrVertexNotFound
	}

	return vertex, nil
}

// LenVertex returns length of vertices.
func (d *dag) LenVertex() int {
	return len(d.vertices)
}

// RangeVertex calls f sequentially for each key and value present in the vertices.
// If f returns false, range stops the iteration.
func (d *dag) RangeVertex(fn func(key string, value *Vertex) bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	for k, v := range d.vertices {
		if !fn(k, v) {
			break
		}
	}
}

// AddEdge adds edge between two vertices.
func (d *dag) AddEdge(fromVertexID, toVertexID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if fromVertexID == toVertexID {
		return ErrCycleBetweenVertices
	}

	fromVertex, ok := d.vertices[fromVertexID]
	if !ok {
		return ErrVertexNotFound
	}

	toVertex, ok := d.vertices[toVertexID]
	if !ok {
		return ErrVertexNotFound
	}

	for _, child := range fromVertex.Children.Values() {
		vertex, ok := child.(*Vertex)
		if !ok {
			continue
		}

		if vertex.ID == toVertexID {
			return ErrCycleBetweenVertices
		}
	}

	if d.depthFirstSearch(toVertexID, fromVertexID) {
		return ErrCycleBetweenVertices
	}

	if ok := fromVertex.Children.Add(toVertex); !ok {
		return ErrChildAlreadyExists
	}

	if ok := toVertex.Parents.Add(fromVertex); !ok {
		return ErrParnetAlreadyExists
	}

	return nil
}

// DeleteEdge deletes edge between two vertices.
func (d *dag) DeleteEdge(fromVertexID, toVertexID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	fromVertex, ok := d.vertices[fromVertexID]
	if !ok {
		return ErrVertexNotFound
	}

	toVertex, ok := d.vertices[toVertexID]
	if !ok {
		return ErrVertexNotFound
	}

	fromVertex.Children.Delete(toVertex)
	toVertex.Parents.Delete(fromVertex)
	return nil
}

// depthFirstSearch is a depth-first search of the directed acyclic graph.
func (d *dag) depthFirstSearch(fromVertexID, toVertexID string) bool {
	successors := make(map[string]struct{})
	d.search(fromVertexID, successors)
	_, ok := successors[toVertexID]
	return ok
}

// depthFirstSearch finds successors of vertex.
func (d *dag) search(vertexID string, successors map[string]struct{}) {
	vertex, ok := d.vertices[vertexID]
	if !ok {
		return
	}

	for _, child := range vertex.Children.Values() {
		vertex, ok := child.(*Vertex)
		if !ok {
			continue
		}

		if _, ok := successors[vertex.ID]; !ok {
			successors[vertex.ID] = struct{}{}
			d.search(vertex.ID, successors)
		}
	}
}
