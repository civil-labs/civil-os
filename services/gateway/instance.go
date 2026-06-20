package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"path/filepath"
	"strings"

	"connectrpc.com/connect"
	instancev1 "github.com/civil-labs/civil-api-go/civil/public/instance/v1"
	"gocloud.dev/blob"
	_ "gocloud.dev/blob/azureblob"
	_ "gocloud.dev/blob/fileblob"
	_ "gocloud.dev/blob/gcsblob"
	_ "gocloud.dev/blob/memblob"
	_ "gocloud.dev/blob/s3blob"
	"google.golang.org/protobuf/types/known/structpb"
)

type InstanceServer struct {
	config Config
	logger *slog.Logger
}

func (s *InstanceServer) GetInstanceMetadata(
	ctx context.Context,
	req *connect.Request[instancev1.GetInstanceMetadataRequest],
) (*connect.Response[instancev1.GetInstanceMetadataResponse], error) {
	s.logger.Debug("received GetInstanceMetadata request")

	uriStr := s.config.InstanceMetadataUri
	if uriStr == "" {
		s.logger.Error("InstanceMetadataUri is empty in config")
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("instance metadata URI is not configured"))
	}

	u, err := url.Parse(uriStr)
	if err != nil {
		s.logger.Error("failed to parse InstanceMetadataUri", slog.String("uri", uriStr), slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("invalid instance metadata URI configuration"))
	}

	var bucketURL string
	var key string

	if u.Scheme == "" {
		bucketURL = "file://" + filepath.Dir(uriStr)
		key = filepath.Base(uriStr)
	} else if u.Scheme == "file" {
		bucketURL = "file://" + filepath.Dir(u.Path)
		key = filepath.Base(u.Path)
	} else {
		bucketURL = u.Scheme + "://" + u.Host
		key = strings.TrimPrefix(u.Path, "/")
	}

	s.logger.Debug("opening bucket for metadata", slog.String("bucketURL", bucketURL), slog.String("key", key))

	bucket, err := blob.OpenBucket(ctx, bucketURL)
	if err != nil {
		s.logger.Error("failed to open metadata bucket", slog.String("bucketURL", bucketURL), slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to open metadata storage"))
	}
	defer bucket.Close()

	data, err := bucket.ReadAll(ctx, key)
	if err != nil {
		s.logger.Error("failed to read metadata key", slog.String("key", key), slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to read metadata file"))
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		s.logger.Error("failed to parse metadata JSON", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("metadata file is not valid JSON"))
	}

	structValue, err := structpb.NewStruct(m)
	if err != nil {
		s.logger.Error("failed to build structpb.Struct from metadata map", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to construct metadata response"))
	}

	res := &instancev1.GetInstanceMetadataResponse{
		Metadata: structValue,
	}

	return connect.NewResponse(res), nil
}
