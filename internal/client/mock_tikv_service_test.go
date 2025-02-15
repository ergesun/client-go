// Copyright 2021 TiKV Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// NOTE: The code in this file is based on code from the
// TiDB project, licensed under the Apache License v 2.0
//
// https://github.com/pingcap/tidb/tree/cc5e161ac06827589c4966674597c137cc9e809c/store/tikv/client/mock_tikv_service_test.go
//

package client

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/pingcap/kvproto/pkg/coprocessor"
	"github.com/pingcap/kvproto/pkg/kvrpcpb"
	"github.com/pingcap/kvproto/pkg/tikvpb"
	"github.com/ergesun/client-go/v2/internal/logutil"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type server struct {
	tikvpb.TikvServer
	grpcServer *grpc.Server
	// metaChecker check the metadata of each request. Now only requests
	// which need redirection set it.
	metaChecker struct {
		sync.Mutex
		check func(context.Context) error
	}
}

func (s *server) KvPrewrite(ctx context.Context, req *kvrpcpb.PrewriteRequest) (*kvrpcpb.PrewriteResponse, error) {
	if err := s.checkMetadata(ctx); err != nil {
		return nil, err
	}
	return &kvrpcpb.PrewriteResponse{}, nil
}

func (s *server) CoprocessorStream(req *coprocessor.Request, ss tikvpb.Tikv_CoprocessorStreamServer) error {
	if err := s.checkMetadata(ss.Context()); err != nil {
		return err
	}
	return ss.Send(&coprocessor.Response{})
}

func (s *server) BatchCommands(ss tikvpb.Tikv_BatchCommandsServer) error {
	if err := s.checkMetadata(ss.Context()); err != nil {
		return err
	}
	for {
		req, err := ss.Recv()
		if err != nil {
			logutil.BgLogger().Error("batch commands receive fail", zap.Error(err))
			return err
		}

		responses := make([]*tikvpb.BatchCommandsResponse_Response, 0, len(req.GetRequestIds()))
		for i := 0; i < len(req.GetRequestIds()); i++ {
			responses = append(responses, &tikvpb.BatchCommandsResponse_Response{
				Cmd: &tikvpb.BatchCommandsResponse_Response_Empty{
					Empty: &tikvpb.BatchCommandsEmptyResponse{},
				},
			})
		}

		err = ss.Send(&tikvpb.BatchCommandsResponse{
			Responses:  responses,
			RequestIds: req.GetRequestIds(),
		})
		if err != nil {
			logutil.BgLogger().Error("batch commands send fail", zap.Error(err))
			return err
		}
	}
}

func (s *server) setMetaChecker(check func(context.Context) error) {
	s.metaChecker.Lock()
	s.metaChecker.check = check
	s.metaChecker.Unlock()
}

func (s *server) checkMetadata(ctx context.Context) error {
	s.metaChecker.Lock()
	defer s.metaChecker.Unlock()
	if s.metaChecker.check != nil {
		return s.metaChecker.check(ctx)
	}
	return nil
}

func (s *server) Stop() {
	s.grpcServer.Stop()
}

// Try to start a gRPC server and retrun the server instance and binded port.
func startMockTikvService() (*server, int) {
	port := -1
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", "127.0.0.1", 0))
	if err != nil {
		logutil.BgLogger().Error("can't listen", zap.Error(err))
		logutil.BgLogger().Error("can't start mock tikv service because no available ports")
		return nil, port
	}
	port = lis.Addr().(*net.TCPAddr).Port

	server := &server{}
	s := grpc.NewServer(grpc.ConnectionTimeout(time.Minute))
	tikvpb.RegisterTikvServer(s, server)
	server.grpcServer = s
	go func() {
		if err = s.Serve(lis); err != nil {
			logutil.BgLogger().Error(
				"can't serve gRPC requests",
				zap.Error(err),
			)
		}
	}()
	return server, port
}
