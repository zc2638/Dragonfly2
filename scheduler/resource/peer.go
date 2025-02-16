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

package resource

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/bits-and-blooms/bitset"
	"github.com/go-http-utils/headers"
	"github.com/looplab/fsm"
	"go.uber.org/atomic"

	logger "d7y.io/dragonfly/v2/internal/dflog"
	"d7y.io/dragonfly/v2/pkg/container/set"
	"d7y.io/dragonfly/v2/pkg/rpc/scheduler"
	"d7y.io/dragonfly/v2/pkg/slices"
)

const (
	// Default value of tag.
	DefaultTag = "unknow"

	// Download tiny file timeout.
	downloadTinyFileContextTimeout = 30 * time.Second
)

const (
	// Peer has been created but did not start running.
	PeerStatePending = "Pending"

	// Peer successfully registered as tiny scope size.
	PeerStateReceivedTiny = "ReceivedTiny"

	// Peer successfully registered as small scope size.
	PeerStateReceivedSmall = "ReceivedSmall"

	// Peer successfully registered as normal scope size.
	PeerStateReceivedNormal = "ReceivedNormal"

	// Peer is downloading resources from peer.
	PeerStateRunning = "Running"

	// Peer is downloading resources from back-to-source.
	PeerStateBackToSource = "BackToSource"

	// Peer has been downloaded successfully.
	PeerStateSucceeded = "Succeeded"

	// Peer has been downloaded failed.
	PeerStateFailed = "Failed"

	// Peer has been left.
	PeerStateLeave = "Leave"
)

const (
	// Peer is registered as tiny scope size.
	PeerEventRegisterTiny = "RegisterTiny"

	// Peer is registered as small scope size.
	PeerEventRegisterSmall = "RegisterSmall"

	// Peer is registered as normal scope size.
	PeerEventRegisterNormal = "RegisterNormal"

	// Peer is downloading.
	PeerEventDownload = "Download"

	// Peer is downloading from back-to-source.
	PeerEventDownloadFromBackToSource = "DownloadFromBackToSource"

	// Peer downloaded successfully.
	PeerEventDownloadSucceeded = "DownloadSucceeded"

	// Peer downloaded failed.
	PeerEventDownloadFailed = "DownloadFailed"

	// Peer leaves.
	PeerEventLeave = "Leave"
)

// PeerOption is a functional option for configuring the peer.
type PeerOption func(p *Peer) *Peer

// WithTag sets peer's Tag.
func WithTag(tag string) PeerOption {
	return func(p *Peer) *Peer {
		p.Tag = tag
		return p
	}
}

type Peer struct {
	// ID is peer id.
	ID string

	// Tag is peer tag.
	Tag string

	// Pieces is piece bitset.
	Pieces *bitset.BitSet

	// pieceCosts is piece downloaded time.
	pieceCosts []int64

	// Stream is grpc stream instance.
	Stream *atomic.Value

	// Task state machine.
	FSM *fsm.FSM

	// Task is peer task.
	Task *Task

	// Host is peer host.
	Host *Host

	// Parent is peer parent.
	Parent *atomic.Value

	// Children is peer children.
	Children *sync.Map

	// ChildCount is child count.
	ChildCount *atomic.Int32

	// StealPeers is steal peer ids.
	StealPeers set.SafeSet

	// BlockPeers is bad peer ids.
	BlockPeers set.SafeSet

	// NeedBackToSource needs downloaded from source.
	//
	// When peer is registering, at the same time,
	// scheduler needs to create the new corresponding task and the seed peer is disabled,
	// NeedBackToSource is set to true.
	NeedBackToSource *atomic.Bool

	// IsBackToSource is downloaded from source.
	//
	// When peer is scheduling and NeedBackToSource is true,
	// scheduler needs to return Code_SchedNeedBackSource and
	// IsBackToSource is set to true.
	IsBackToSource *atomic.Bool

	// CreateAt is peer create time.
	CreateAt *atomic.Time

	// UpdateAt is peer update time.
	UpdateAt *atomic.Time

	// Peer mutex.
	mu *sync.RWMutex

	// Peer log.
	Log *logger.SugaredLoggerOnWith
}

// New Peer instance.
func NewPeer(id string, task *Task, host *Host, options ...PeerOption) *Peer {
	p := &Peer{
		ID:               id,
		Tag:              DefaultTag,
		Pieces:           &bitset.BitSet{},
		pieceCosts:       []int64{},
		Stream:           &atomic.Value{},
		Task:             task,
		Host:             host,
		Parent:           &atomic.Value{},
		Children:         &sync.Map{},
		ChildCount:       atomic.NewInt32(0),
		StealPeers:       set.NewSafeSet(),
		BlockPeers:       set.NewSafeSet(),
		NeedBackToSource: atomic.NewBool(false),
		IsBackToSource:   atomic.NewBool(false),
		CreateAt:         atomic.NewTime(time.Now()),
		UpdateAt:         atomic.NewTime(time.Now()),
		mu:               &sync.RWMutex{},
		Log:              logger.WithTaskAndPeerID(task.ID, id),
	}

	// Initialize state machine.
	p.FSM = fsm.NewFSM(
		PeerStatePending,
		fsm.Events{
			{Name: PeerEventRegisterTiny, Src: []string{PeerStatePending}, Dst: PeerStateReceivedTiny},
			{Name: PeerEventRegisterSmall, Src: []string{PeerStatePending}, Dst: PeerStateReceivedSmall},
			{Name: PeerEventRegisterNormal, Src: []string{PeerStatePending}, Dst: PeerStateReceivedNormal},
			{Name: PeerEventDownload, Src: []string{PeerStateReceivedTiny, PeerStateReceivedSmall, PeerStateReceivedNormal}, Dst: PeerStateRunning},
			{Name: PeerEventDownloadFromBackToSource, Src: []string{PeerStateReceivedTiny, PeerStateReceivedSmall, PeerStateReceivedNormal, PeerStateRunning}, Dst: PeerStateBackToSource},
			{Name: PeerEventDownloadSucceeded, Src: []string{
				// Since ReportPeerResult and ReportPieceResult are called in no order,
				// the result may be reported after the register is successful.
				PeerStateReceivedTiny, PeerStateReceivedSmall, PeerStateReceivedNormal,
				PeerStateRunning, PeerStateBackToSource,
			}, Dst: PeerStateSucceeded},
			{Name: PeerEventDownloadFailed, Src: []string{
				PeerStatePending, PeerStateReceivedTiny, PeerStateReceivedSmall, PeerStateReceivedNormal,
				PeerStateRunning, PeerStateBackToSource, PeerStateSucceeded,
			}, Dst: PeerStateFailed},
			{Name: PeerEventLeave, Src: []string{PeerStateFailed, PeerStateSucceeded}, Dst: PeerEventLeave},
		},
		fsm.Callbacks{
			PeerEventRegisterTiny: func(e *fsm.Event) {
				p.UpdateAt.Store(time.Now())
				p.Log.Infof("peer state is %s", e.FSM.Current())
			},
			PeerEventRegisterSmall: func(e *fsm.Event) {
				p.UpdateAt.Store(time.Now())
				p.Log.Infof("peer state is %s", e.FSM.Current())
			},
			PeerEventRegisterNormal: func(e *fsm.Event) {
				p.UpdateAt.Store(time.Now())
				p.Log.Infof("peer state is %s", e.FSM.Current())
			},
			PeerEventDownload: func(e *fsm.Event) {
				p.UpdateAt.Store(time.Now())
				p.Log.Infof("peer state is %s", e.FSM.Current())
			},
			PeerEventDownloadFromBackToSource: func(e *fsm.Event) {
				p.IsBackToSource.Store(true)
				p.Task.BackToSourcePeers.Add(p)
				p.DeleteParent()
				p.Host.DeletePeer(p.ID)
				p.UpdateAt.Store(time.Now())
				p.Log.Infof("peer state is %s", e.FSM.Current())
			},
			PeerEventDownloadSucceeded: func(e *fsm.Event) {
				if e.Src == PeerStateBackToSource {
					p.Task.BackToSourcePeers.Delete(p)
				}

				p.DeleteParent()
				p.Host.DeletePeer(p.ID)
				p.Task.PeerFailedCount.Store(0)
				p.UpdateAt.Store(time.Now())
				p.Log.Infof("peer state is %s", e.FSM.Current())
			},
			PeerEventDownloadFailed: func(e *fsm.Event) {
				if e.Src == PeerStateBackToSource {
					p.Task.PeerFailedCount.Inc()
					p.Task.BackToSourcePeers.Delete(p)
				}

				p.DeleteParent()
				p.Host.DeletePeer(p.ID)
				p.UpdateAt.Store(time.Now())
				p.Log.Infof("peer state is %s", e.FSM.Current())
			},
			PeerEventLeave: func(e *fsm.Event) {
				p.DeleteParent()
				p.Host.DeletePeer(p.ID)
				p.Log.Infof("peer state is %s", e.FSM.Current())
			},
		},
	)

	for _, opt := range options {
		opt(p)
	}

	return p
}

// LoadChild return peer child for a key.
func (p *Peer) LoadChild(key string) (*Peer, bool) {
	rawChild, ok := p.Children.Load(key)
	if !ok {
		return nil, false
	}

	return rawChild.(*Peer), ok
}

// StoreChild set peer child.
func (p *Peer) StoreChild(child *Peer) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, loaded := p.Children.LoadOrStore(child.ID, child); !loaded {
		p.ChildCount.Inc()
		p.Host.UploadPeerCount.Inc()
	}

	child.Parent.Store(p)
}

// DeleteChild deletes peer child for a key.
func (p *Peer) DeleteChild(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	child, ok := p.LoadChild(key)
	if !ok {
		return
	}

	if _, loaded := p.Children.LoadAndDelete(child.ID); loaded {
		p.ChildCount.Dec()
		p.Host.UploadPeerCount.Dec()
	}

	child.Parent = &atomic.Value{}
}

// LoadParent return peer parent.
func (p *Peer) LoadParent() (*Peer, bool) {
	rawParent := p.Parent.Load()
	if rawParent == nil {
		return nil, false
	}

	return rawParent.(*Peer), true
}

// StoreParent set peer parent.
func (p *Peer) StoreParent(parent *Peer) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.Parent.Store(parent)
	if _, loaded := parent.Children.LoadOrStore(p.ID, p); !loaded {
		parent.ChildCount.Inc()
		parent.Host.UploadPeerCount.Inc()
	}
}

// DeleteParent deletes peer parent.
func (p *Peer) DeleteParent() {
	p.mu.Lock()
	defer p.mu.Unlock()

	parent, ok := p.LoadParent()
	if !ok {
		return
	}
	p.Parent = &atomic.Value{}

	if _, loaded := parent.Children.LoadAndDelete(p.ID); loaded {
		parent.ChildCount.Dec()
		parent.Host.UploadPeerCount.Dec()
	}
}

// ReplaceParent replaces peer parent.
func (p *Peer) ReplaceParent(parent *Peer) {
	p.DeleteParent()
	p.StoreParent(parent)
}

// Depth represents depth of tree.
func (p *Peer) Depth() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var (
		node      = p
		ancestors = []string{p.ID}
	)
	for node != nil {
		if node.Host.Type != HostTypeNormal {
			break
		}

		parent, ok := node.LoadParent()
		if !ok {
			break
		}

		ancestors = append(ancestors, parent.ID)

		// Prevent traversal tree from infinite loop.
		if _, found := slices.FindDuplicate(ancestors); found {
			p.Log.Error("tree structure produces an infinite loop")
			break
		}

		node = parent
	}

	return len(ancestors)
}

// Ancestors returns peer's ancestors.
func (p *Peer) Ancestors() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var (
		node      = p
		ancestors = []string{p.ID}
	)
	for node != nil {
		parent, ok := node.LoadParent()
		if !ok {
			return ancestors
		}

		ancestors = append(ancestors, parent.ID)

		// Prevent traversal tree from infinite loop.
		if _, found := slices.FindDuplicate(ancestors); found {
			p.Log.Error("tree structure produces an infinite loop")
			break
		}

		node = parent
	}

	return ancestors
}

// IsDescendant determines whether it is ancestor of peer.
func (p *Peer) IsDescendant(ancestor *Peer) bool {
	return ancestor.isDescendant(p)
}

// IsAncestor determines whether it is descendant of peer.
func (p *Peer) IsAncestor(descendant *Peer) bool {
	return p.isDescendant(descendant)
}

// isDescendant determines whether it is ancestor of peer.
func (p *Peer) isDescendant(descendant *Peer) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var (
		node      = descendant
		ancestors = []string{descendant.ID}
	)
	for node != nil {
		parent, ok := node.LoadParent()
		if !ok {
			return false
		}

		if parent.ID == p.ID {
			return true
		}

		ancestors = append(ancestors, parent.ID)

		// Prevent traversal tree from infinite loop.
		if _, found := slices.FindDuplicate(ancestors); found {
			p.Log.Error("tree structure produces an infinite loop")
			break
		}

		node = parent
	}

	return false
}

// AppendPieceCost append piece cost to costs slice.
func (p *Peer) AppendPieceCost(cost int64) {
	p.pieceCosts = append(p.pieceCosts, cost)
}

// PieceCosts return piece costs slice.
func (p *Peer) PieceCosts() []int64 {
	return p.pieceCosts
}

// LoadStream return grpc stream.
func (p *Peer) LoadStream() (scheduler.Scheduler_ReportPieceResultServer, bool) {
	rawStream := p.Stream.Load()
	if rawStream == nil {
		return nil, false
	}

	return rawStream.(scheduler.Scheduler_ReportPieceResultServer), true
}

// StoreStream set grpc stream.
func (p *Peer) StoreStream(stream scheduler.Scheduler_ReportPieceResultServer) {
	p.Stream.Store(stream)
}

// DeleteStream deletes grpc stream.
func (p *Peer) DeleteStream() {
	p.Stream = &atomic.Value{}
}

// DownloadTinyFile downloads tiny file from peer.
func (p *Peer) DownloadTinyFile() ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), downloadTinyFileContextTimeout)
	defer cancel()

	// Download url: http://${host}:${port}/download/${taskIndex}/${taskID}?peerId=${peerID}
	targetURL := url.URL{
		Scheme:   "http",
		Host:     fmt.Sprintf("%s:%d", p.Host.IP, p.Host.DownloadPort),
		Path:     fmt.Sprintf("download/%s/%s", p.Task.ID[:3], p.Task.ID),
		RawQuery: fmt.Sprintf("peerId=%s", p.ID),
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL.String(), nil)
	if err != nil {
		return []byte{}, err
	}

	req.Header.Set(headers.Range, fmt.Sprintf("bytes=%d-%d", 0, p.Task.ContentLength.Load()-1))
	p.Log.Infof("download tiny file %s, header is : %#v", targetURL.String(), req.Header)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// The HTTP 206 Partial Content success status response code indicates that
	// the request has succeeded and the body contains the requested ranges of data, as described in the Range header of the request.
	// Refer to https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/206.
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("bad response status %s", resp.Status)
	}

	return io.ReadAll(resp.Body)
}
