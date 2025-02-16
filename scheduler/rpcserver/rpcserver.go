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

package rpcserver

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	empty "google.golang.org/protobuf/types/known/emptypb"

	"d7y.io/dragonfly/v2/pkg/idgen"
	"d7y.io/dragonfly/v2/pkg/rpc"
	"d7y.io/dragonfly/v2/pkg/rpc/scheduler"
	"d7y.io/dragonfly/v2/scheduler/metrics"
	"d7y.io/dragonfly/v2/scheduler/resource"
	"d7y.io/dragonfly/v2/scheduler/service"
)

// Server is grpc server.
type Server struct {
	// Service interface.
	service *service.Service

	// GRPC UnimplementedSchedulerServer interface.
	scheduler.UnimplementedSchedulerServer
}

// New returns a new transparent scheduler server from the given options.
func New(service *service.Service, opts ...grpc.ServerOption) *grpc.Server {
	svr := &Server{service: service}
	grpcServer := grpc.NewServer(append(rpc.DefaultServerOptions(), opts...)...)

	// Register servers on grpc server.
	scheduler.RegisterSchedulerServer(grpcServer, svr)
	healthpb.RegisterHealthServer(grpcServer, health.NewServer())
	return grpcServer
}

// RegisterPeerTask registers peer and triggers seed peer download task.
func (s *Server) RegisterPeerTask(ctx context.Context, req *scheduler.PeerTaskRequest) (*scheduler.RegisterResult, error) {
	// FIXME: Scheudler will not generate task id.
	if req.TaskId == "" {
		req.TaskId = idgen.TaskID(req.Url, req.UrlMeta)
	}

	tag := resource.DefaultTag
	if req.UrlMeta.Tag != "" {
		tag = req.UrlMeta.Tag
	}
	metrics.RegisterPeerTaskCount.WithLabelValues(tag).Inc()

	resp, err := s.service.RegisterPeerTask(ctx, req)
	if err != nil {
		metrics.RegisterPeerTaskFailureCount.WithLabelValues(tag).Inc()
	} else {
		metrics.PeerTaskCounter.WithLabelValues(tag, resp.SizeScope.String()).Inc()
	}

	return resp, err
}

// ReportPieceResult handles the piece information reported by dfdaemon.
func (s *Server) ReportPieceResult(stream scheduler.Scheduler_ReportPieceResultServer) error {
	metrics.ConcurrentScheduleGauge.Inc()
	defer metrics.ConcurrentScheduleGauge.Dec()

	return s.service.ReportPieceResult(stream)
}

// ReportPeerResult handles peer result reported by dfdaemon.
func (s *Server) ReportPeerResult(ctx context.Context, req *scheduler.PeerResult) (*empty.Empty, error) {
	return new(empty.Empty), s.service.ReportPeerResult(ctx, req)
}

// StatTask checks if the given task exists.
func (s *Server) StatTask(ctx context.Context, req *scheduler.StatTaskRequest) (*scheduler.Task, error) {
	metrics.StatTaskCount.Inc()
	task, err := s.service.StatTask(ctx, req)
	if err != nil {
		metrics.StatTaskFailureCount.Inc()
		return nil, err
	}

	return task, nil
}

// AnnounceTask informs scheduler a peer has completed task.
func (s *Server) AnnounceTask(ctx context.Context, req *scheduler.AnnounceTaskRequest) (*empty.Empty, error) {
	metrics.AnnounceCount.Inc()
	if err := s.service.AnnounceTask(ctx, req); err != nil {
		metrics.AnnounceFailureCount.Inc()
		return new(empty.Empty), err
	}

	return new(empty.Empty), nil
}

// LeaveTask makes the peer unschedulable.
func (s *Server) LeaveTask(ctx context.Context, req *scheduler.PeerTarget) (*empty.Empty, error) {
	return new(empty.Empty), s.service.LeaveTask(ctx, req)
}
