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
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
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
		Tag:          "v1.21.0", // Replace this with an actual version
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

func TestCreateExistingRelationshipWithoutPrecondition(t *testing.T) {
	client, err := NewConnection()
	assert.NoError(t, err)

	_, err = client.WriteRelationships(context.Background(), &v1.WriteRelationshipsRequest{
		Updates: []*v1.RelationshipUpdate{{
			Operation: v1.RelationshipUpdate_OPERATION_CREATE,
			Relationship: &v1.Relationship{
				Resource: &v1.ObjectReference{
					ObjectType: "access",
					ObjectId:   "blue",
				},
				Relation: "assigned",
				Subject: &v1.SubjectReference{
					Object: &v1.ObjectReference{
						ObjectType: "user",
						ObjectId:   "alice",
					},
				},
			},
		}},
	})

	assertDetailedError(t, err)
}

func TestCreateExistingRelationshipWithPrecondition(t *testing.T) {
	client, err := NewConnection()
	assert.NoError(t, err)

	_, err = client.WriteRelationships(context.Background(), &v1.WriteRelationshipsRequest{
		Updates: []*v1.RelationshipUpdate{{
			Operation: v1.RelationshipUpdate_OPERATION_CREATE,
			Relationship: &v1.Relationship{
				Resource: &v1.ObjectReference{
					ObjectType: "access",
					ObjectId:   "blue",
				},
				Relation: "assigned",
				Subject: &v1.SubjectReference{
					Object: &v1.ObjectReference{
						ObjectType: "user",
						ObjectId:   "alice",
					},
				},
			},
		}},
		OptionalPreconditions: []*v1.Precondition{{
			Operation: v1.Precondition_OPERATION_MUST_NOT_MATCH,
			Filter: &v1.RelationshipFilter{
				ResourceType:       "access",
				OptionalResourceId: "blue",
				OptionalRelation:   "assigned",
				OptionalSubjectFilter: &v1.SubjectFilter{
					SubjectType:       "user",
					OptionalSubjectId: "alice",
				},
			},
		}},
	})

	assertDetailedError(t, err)
}

func assertDetailedError(t *testing.T, err error) {
	assert.Error(t, err)

	if s, ok := status.FromError(err); ok {
		if len(s.Details()) > 0 {
			if info := s.Details()[0].(*errdetails.ErrorInfo); ok {
				t.Logf("Got detailed info: %v", info)
			} else {
				assert.Failf(t, "Detail element not ErrorInfo", "Detail elements present but unable to coerce to ErrorInfo ptr. Actual: %v", s.Details())
			}
		} else {
			assert.Failf(t, "No detail elements", "Status has no details: %v", s)
		}
	} else {
		assert.Failf(t, "No status for error", "Unable to extract status from error: %v")
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
