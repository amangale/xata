package strategy_test

import (
	"testing"

	"xata/services/projects/scheduler/strategy"

	"github.com/stretchr/testify/require"
)

func TestToStrategy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		strategyName     strategy.Name
		expectedStrategy strategy.Interface
		expectedError    error
	}{
		{
			name:             "valid strategy - AlwaysPrimary",
			strategyName:     strategy.AlwaysPrimaryStrategyName,
			expectedStrategy: &strategy.AlwaysPrimary{},
			expectedError:    nil,
		},
		{
			name:             "valid strategy - AlwaysSecondary",
			strategyName:     strategy.AlwaysSecondaryStrategyName,
			expectedStrategy: &strategy.AlwaysSecondary{},
			expectedError:    nil,
		},
		{
			name:             "valid strategy - Random",
			strategyName:     strategy.RandomStrategyName,
			expectedStrategy: &strategy.Random{},
			expectedError:    nil,
		},
		{
			name:             "invalid strategy",
			strategyName:     "InvalidStrategy",
			expectedStrategy: nil,
			expectedError:    strategy.ErrInvalidStrategy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, err := tt.strategyName.ToStrategy()
			if tt.expectedError != nil {
				require.ErrorIs(t, err, tt.expectedError)
			} else {
				require.NoError(t, err)
				require.IsType(t, tt.expectedStrategy, s)
			}
		})
	}
}
