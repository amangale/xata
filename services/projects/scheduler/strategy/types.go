package strategy

import (
	"context"
	"errors"
	"fmt"

	"xata/services/projects/store"
)

type Name string

const (
	AlwaysPrimaryStrategyName   Name = "AlwaysPrimary"
	AlwaysSecondaryStrategyName Name = "AlwaysSecondary"
	RandomStrategyName          Name = "Random"
)

var ErrInvalidStrategy = errors.New("invalid strategy")

// Interface defines the interface for a scheduling strategy.
type Interface interface {
	// Schedule selects a cell from the provided list of cells.
	Schedule(ctx context.Context, cells []store.Cell) (*store.Cell, error)
}

// ToStrategy converts a Name to its corresponding strategy implementation.
func (s Name) ToStrategy() (Interface, error) {
	switch s {
	case AlwaysPrimaryStrategyName:
		return &AlwaysPrimary{}, nil
	case AlwaysSecondaryStrategyName:
		return &AlwaysSecondary{}, nil
	case RandomStrategyName:
		return &Random{}, nil
	default:
		return nil, fmt.Errorf("%w - %q", ErrInvalidStrategy, s)
	}
}
