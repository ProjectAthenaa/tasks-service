package resolvers

import (
	"context"
	"github.com/ProjectAthenaa/sonic-core/protos/module"
	"github.com/ProjectAthenaa/tasks-service/graph/model"
	"github.com/go-redis/redis/v8"
	"github.com/prometheus/common/log"
)

func (r *Resolver) handleUpdates(ctx context.Context, updates chan *model.TaskStatus, subscription *redis.PubSub) {
	defer close(updates)
	defer func(subscription *redis.PubSub) {
		if err := subscription.Close(); err != nil {
			log.Error("error closing subscription: ", err)
		}
	}(subscription)


	for {
		select {
		case <-ctx.Done():
			log.Info("Subscription Closed")
			return
		case update, ok := <-subscription.Channel():
			if !ok {
				return
			}
			go r.processUpdate(updates, update.Payload)
		}
	}
}

func (r *Resolver) processUpdate(updates chan *model.TaskStatus, payload string) {
	var update *module.Status

	if err := json.Unmarshal([]byte(payload), &update); err != nil {
		log.Errorf("error unmarshalling payload: ", err)
		return
	}

	updates <- convertUpdate(update)
}

func convertUpdate(status *module.Status) *model.TaskStatus {
	returningStatus := &model.TaskStatus{
		TaskID:      status.Information["taskID"],
		Status:      model.Status(module.STATUS_name[int32(status.Status)]),
		Error:       status.Error,
		Information: map[string]interface{}{},
	}

	for k, v := range status.Information {
		if k == "taskID" || k == "stoppedFromCMD" {
			continue
		}
		returningStatus.Information[k] = v
	}

	return returningStatus
}
