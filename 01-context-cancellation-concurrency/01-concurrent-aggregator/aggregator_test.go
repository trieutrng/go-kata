package concurrent_aggregator

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"trieutrng.com/go-kata/concurrent-aggregator/order"
	"trieutrng.com/go-kata/concurrent-aggregator/profile"
)

type MockProfileService struct {
	profiles map[int]*profile.Profile
	err      error
	delay    time.Duration
}

func (p *MockProfileService) Get(ctx context.Context, id int) (*profile.Profile, error) {
	select {
	case <-ctx.Done():
		return nil, ErrTimeout
	case <-time.After(p.delay):
	}

	if p.err != nil {
		return nil, p.err
	}

	return p.profiles[id], nil
}

type MockOrderService struct {
	orders map[int][]*order.Order
	err    error
	delay  time.Duration
}

func (o *MockOrderService) GetAll(ctx context.Context, userId int) ([]*order.Order, error) {
	select {
	case <-ctx.Done():
		return nil, ErrTimeout
	case <-time.After(o.delay):
	}

	if o.err != nil {
		return nil, o.err
	}

	return o.orders[userId], nil
}

func TestAggregator(t *testing.T) {
	type Input struct {
		profileService profile.Service
		orderService   order.Service
		profileId      int
		timeout        time.Duration
		parentTimeout  time.Duration
	}
	type Expected struct {
		aggregatedResult *AggregatedResult
		err              error
	}
	type TestCase struct {
		name     string
		input    Input
		expected Expected
	}

	testProfiles := map[int]*profile.Profile{
		1: {
			Id:   1,
			Name: "user 1",
		},
		2: {
			Id:   2,
			Name: "user 2",
		},
		3: {
			Id:   3,
			Name: "user 3",
		},
	}

	testOrders := map[int][]*order.Order{
		1: {
			{
				Id:     1,
				UserId: 1,
			},
			{
				Id:     2,
				UserId: 1,
			},
			{
				Id:     3,
				UserId: 1,
			},
		},
		2: {
			{
				Id:     4,
				UserId: 2,
			},
		},
		3: {},
	}

	testCases := []TestCase{
		{
			name: "no error, aggregate successful",
			input: Input{
				profileService: &MockProfileService{
					profiles: testProfiles,
					err:      nil,
					delay:    2 * time.Second,
				},
				orderService: &MockOrderService{
					orders: testOrders,
					err:    nil,
					delay:  1 * time.Second,
				},
				profileId:     1,
				timeout:       3 * time.Second,
				parentTimeout: 5 * time.Second,
			},
			expected: Expected{
				aggregatedResult: &AggregatedResult{
					profile: &profile.Profile{
						Id:   1,
						Name: "user 1",
					},
					orders: []*order.Order{
						{
							Id:     1,
							UserId: 1,
						},
						{
							Id:     2,
							UserId: 1,
						},
						{
							Id:     3,
							UserId: 1,
						},
					},
				},
				err: nil,
			},
		},
		{
			name: "context deadline exceeded error",
			input: Input{
				profileService: &MockProfileService{
					profiles: testProfiles,
					err:      nil,
					delay:    1 * time.Second,
				},
				orderService: &MockOrderService{
					orders: testOrders,
					err:    nil,
					delay:  2 * time.Second,
				},
				profileId:     2,
				timeout:       1 * time.Second,
				parentTimeout: 5 * time.Second,
			},
			expected: Expected{
				aggregatedResult: nil,
				err:              ErrTimeout,
			},
		},
		{
			name: "immediate profile service error",
			input: Input{
				profileService: &MockProfileService{
					profiles: testProfiles,
					err:      errors.New("Profile service error"),
					delay:    0 * time.Second,
				},
				orderService: &MockOrderService{
					orders: testOrders,
					err:    errors.New("Order service error"),
					delay:  1 * time.Second,
				},
				profileId:     2,
				timeout:       3 * time.Second,
				parentTimeout: 5 * time.Second,
			},
			expected: Expected{
				aggregatedResult: nil,
				err:              errors.New("Profile service error"),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.input.parentTimeout != 0 {
				timeoutCtx, cancel := context.WithTimeout(ctx, tc.input.parentTimeout)
				ctx = timeoutCtx
				defer cancel()
			}

			var buf bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&buf, nil))

			aggregator := NewUserAggregator(
				WithProfileService(tc.input.profileService),
				WithOrderService(tc.input.orderService),
				WithTimeOut(tc.input.timeout),
				WithLogger(logger),
			)

			result, err := aggregator.Aggregate(ctx, tc.input.profileId)

			assert.Equal(t, tc.expected.err, err, "Expected error: %v - Actual error: %v", tc.expected.err, err)
			assert.Equal(t, tc.expected.aggregatedResult, result)
		})
	}
}
