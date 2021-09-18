package resolvers

import (
	protos "github.com/ProjectAthenaa/scheduler-service/connector"
	"github.com/ProjectAthenaa/sonic-core/sonic/core"
	"github.com/ProjectAthenaa/sonic-core/sonic/database/ent"
	"github.com/go-redis/redis/v8"
	"google.golang.org/grpc"
	"os"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	conn      redis.UniversalClient
	client    *ent.Client
	scheduler protos.SchedulerClient
}

//NewResolver creates a new instance of Resolver with the necessary fields
func NewResolver() (resolver *Resolver, err error) {
	var conn *grpc.ClientConn

	if os.Getenv("DEBUG") == "1" {
		conn, err = grpc.Dial("localhost:6000", grpc.WithInsecure())
		if err != nil {
			return nil, err
		}
	} else {
		conn, err = grpc.Dial("scheduler.general.svc.cluster.local:3000", grpc.WithInsecure())
		if err != nil {
			return nil, err
		}
	}

	return &Resolver{
		conn:      core.Base.GetRedis("cache"),
		client:    core.Base.GetPg("pg"),
		scheduler: protos.NewSchedulerClient(conn),
	}, nil
}
