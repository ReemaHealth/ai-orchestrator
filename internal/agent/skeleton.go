package agent

import (
	"context"
	"fmt"
)

// SkeletonClient emits placeholder chunks for local development without Vertex credentials.
type SkeletonClient struct{}

// NewSkeletonClient returns a Client that streams fixed skeleton token chunks.
func NewSkeletonClient() *SkeletonClient {
	return &SkeletonClient{}
}

// StreamQuery implements Client with five placeholder chunks.
func (c *SkeletonClient) StreamQuery(_ context.Context, _ StreamQueryInput, emit func(chunk string) error) error {
	for i := 1; i <= 5; i++ {
		if err := emit(fmt.Sprintf("Skeleton token chunk %d", i)); err != nil {
			return err
		}
	}
	return nil
}
