package resolvers

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"errors"
	"fmt"
	"time"

	protos "github.com/ProjectAthenaa/scheduler-service/connector"
	"github.com/ProjectAthenaa/sonic-core/sonic"
	"github.com/ProjectAthenaa/sonic-core/sonic/database/ent"
	task2 "github.com/ProjectAthenaa/sonic-core/sonic/database/ent/task"
	"github.com/ProjectAthenaa/tasks-service/graph/generated"
	"github.com/ProjectAthenaa/tasks-service/graph/model"
	"github.com/prometheus/common/log"
)

func (r *mutationResolver) SendCommand(ctx context.Context, controlToken string, command model.Command) (bool, error) {
	if _, err := contextExtract(ctx); err != nil {
		return false, err
	}

	//publish command to given redis channel
	if err := r.conn.Publish(ctx, controlToken, command).Err(); err != nil {
		return false, err
	}

	return true, nil
}

func (r *mutationResolver) StartTasks(ctx context.Context, taskIDs []string) (bool, error) {
	_, err := contextExtract(ctx)
	if err != nil {
		return false, err
	}

	//create pipeline to bulk update redis
	pipe := r.conn.Pipeline()

	var updates []*ent.TaskUpdateOne

	//get current start time
	startTime := time.Now()

	for _, id := range taskIDs {
		log.Info("Task: ", id, " Processing: ", r.isBeingProcessed(ctx, id))
		if r.isBeingProcessed(ctx, id) {
			continue
		}
		//retrieve each task in order to get necessary data
		task, err := r.client.
			Task.
			Query().
			WithProduct().
			Where(
				task2.ID(
					sonic.UUIDParser(id),
				),
			).First(ctx)
		if err != nil {
			log.Error("error retrieving task: ", err)
			continue
		}

		//create an update builder to use later to update the db
		updates = append(updates, task.Update().SetStartTime(startTime))

		//append the data to the queue
		pipe.RPush(ctx, "queue:"+string(task.Edges.Product[0].Site), task.ID.String())
	}

	if _, err = pipe.Exec(ctx); err != nil {
		return false, err
	}

	go func() {
		//sleep so that the hook doesn't re-update the tasks
		time.Sleep(time.Second * 5)
		for _, update := range updates {
			if _, err = update.Save(context.Background()); err != nil {
				log.Error("error updating task: ", err)
				continue
			}
		}
	}()

	return true, nil
}

func (r *queryResolver) GetScheduledTasks(ctx context.Context) ([]*model.Task, error) {
	//get UserID from context
	userID, err := contextExtract(ctx)
	if err != nil {
		return nil, err
	}

	//get scheduled tasks from the scheduler, will be a random instance
	tasks, err := r.scheduler.GetScheduledTasks(ctx, &protos.User{ID: *userID})
	if err != nil {
		log.Error("error getting scheduled tasks: ", err)
		return nil, errors.New("internal_error")
	}

	var convertedTasks []*model.Task

	//convert tasks from connector.Tasks to an array of model.Task
	for _, task := range tasks.Tasks {
		convertedTasks = append(convertedTasks, &model.Task{
			ID:                task.ID,
			SubscriptionToken: task.SubscriptionToken,
			ControlToken:      task.ControlToken,
			StartTime:         time.Unix(task.StartTime, 0),
		})
	}

	return convertedTasks, nil
}

func (r *queryResolver) GetRunningTasks(ctx context.Context) ([]*model.Task, error) {
	userID, err := contextExtract(ctx)
	if err != nil {
		return nil, err
	}

	user, err := r.client.User.Get(ctx, sonic.UUIDParser(*userID))
	if err != nil {
		return nil, sonic.EntErr(err)
	}

	app, err := user.App(ctx)
	if err != nil {
		return nil, sonic.EntErr(err)
	}

	tasks := app.QueryTaskGroups().QueryTasks().AllX(ctx)

	var runningTasks []*model.Task

	for _, taskID := range r.conn.SMembers(ctx, "tasks:processing").Val() {
		for _, task := range tasks {
			if taskID == task.ID.String() {
				runningTasks = append(runningTasks, &model.Task{
					ID:                taskID,
					SubscriptionToken: taskID,
					ControlToken:      hash(taskID),
					StartTime:         *task.StartTime,
					Status:            model.Status(r.conn.Get(ctx, fmt.Sprintf("tasks:updates:last-update:%s", taskID)).Val()),
				})
			}
		}
	}

	return runningTasks, nil
}

func (r *subscriptionResolver) TaskUpdates(ctx context.Context, subscriptionTokens []string) (<-chan *model.TaskStatus, error) {
	//the channel in which updates are pushed to
	updates := make(chan *model.TaskStatus)

	var channels []string

	for _, token := range subscriptionTokens {
		channels = append(channels, fmt.Sprintf("tasks:updates:%s", token))
	}

	//updates subscription
	subscription := r.conn.Subscribe(ctx, channels...)

	//handle new updates
	go r.handleUpdates(ctx, updates, subscription)

	return updates, nil
}

// Mutation returns generated.MutationResolver implementation.
func (r *Resolver) Mutation() generated.MutationResolver { return &mutationResolver{r} }

// Query returns generated.QueryResolver implementation.
func (r *Resolver) Query() generated.QueryResolver { return &queryResolver{r} }

// Subscription returns generated.SubscriptionResolver implementation.
func (r *Resolver) Subscription() generated.SubscriptionResolver { return &subscriptionResolver{r} }

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
type subscriptionResolver struct{ *Resolver }
