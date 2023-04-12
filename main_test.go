package main

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"testing"

	v1 "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"github.com/authzed/authzed-go/v1"
	"github.com/authzed/grpcutil"
	"github.com/google/uuid"
	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var port string

func TestMain(m *testing.M) {
	pool, err := dockertest.NewPool("") // Empty string uses default docker env
	if err != nil {
		os.Exit(1)
	}

	var (
		_, b, _, _ = runtime.Caller(0)
		basepath   = filepath.Dir(b)
	)

	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository:   "authzed/spicedb",
		Tag:          "v1.17.0", // Replace this with an actual version
		Cmd:          []string{"serve-testing", "--load-configs", "/mnt/spicedb_bootstrap.yaml"},
		Mounts:       []string{path.Join(basepath, "spicedb_bootstrap.yaml") + ":/mnt/spicedb_bootstrap.yaml"},
		ExposedPorts: []string{"50051/tcp"},
	})
	if err != nil {
		os.Exit(1)
	}

	port = resource.GetPort("50051/tcp")

	result := m.Run()
	_ = pool.Purge(resource)

	os.Exit(result)
}

func TestCheckPermission(t *testing.T) {
	for i := 0; i < 1000; i++ {
		client, err := NewConnection()
		assert.NoError(t, err)

		result, err := client.CheckPermission(context.Background(), &v1.CheckPermissionRequest{
			Resource: &v1.ObjectReference{
				ObjectType: "access",
				ObjectId:   "blue",
			},
			Permission: "assigned",
			Subject: &v1.SubjectReference{
				Object: &v1.ObjectReference{
					ObjectType: "user",
					ObjectId:   "alice",
				},
			},
		})

		assert.NoError(t, err)

		assert.Equal(t, v1.CheckPermissionResponse_PERMISSIONSHIP_HAS_PERMISSION, result.Permissionship, "Error on attempt #%d", i)
	}
}

func NewConnection() (*authzed.Client, error) {
	token := uuid.New().String()

	opts := []grpc.DialOption{
		grpcutil.WithInsecureBearerToken(token),
		grpc.WithBlock(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	return authzed.NewClient(
		"localhost:"+port,
		opts...,
	)
}
