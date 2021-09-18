package resolvers

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigFastest

//contextExtract extracts a potential error and userID from the context
func contextExtract(ctx context.Context) (*string, error) {
	if err := ctx.Value("error"); err != nil {
		return nil, err.(error)
	}

	if userID := ctx.Value("userID"); userID != nil {
		id := fmt.Sprint(userID)
		return &id, nil
	}

	return nil, errors.New("user_not_found")
}

func (r *Resolver) isBeingProcessed(ctx context.Context, taskID string) bool {
	return r.conn.SIsMember(ctx, "tasks:processing", taskID).Val()
}

func hash(text string) string {
	algorithm := sha1.New()
	algorithm.Write([]byte(text))
	return hex.EncodeToString(algorithm.Sum(nil))
}