package concurrent_aggregator

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"golang.org/x/sync/errgroup"
	"trieutrng.com/go-kata/concurrent-aggregator/order"
	"trieutrng.com/go-kata/concurrent-aggregator/profile"
)

var ErrTimeout = errors.New("context deadline exceeded")

type UserAggregator struct {
	timeout        time.Duration
	logger         *slog.Logger
	profileService profile.Service
	orderService   order.Service
}

func NewUserAggregator(withParams ...func(u *UserAggregator)) *UserAggregator {
	u := &UserAggregator{}
	for _, w := range withParams {
		w(u)
	}
	return u
}

func WithTimeOut(timeout time.Duration) func(u *UserAggregator) {
	return func(u *UserAggregator) {
		u.timeout = timeout
	}
}

func WithLogger(logger *slog.Logger) func(u *UserAggregator) {
	return func(u *UserAggregator) {
		u.logger = logger
	}
}

func WithProfileService(profileService profile.Service) func(u *UserAggregator) {
	return func(u *UserAggregator) {
		u.profileService = profileService
	}
}

func WithOrderService(orderService order.Service) func(u *UserAggregator) {
	return func(u *UserAggregator) {
		u.orderService = orderService
	}
}

type AggregatedResult struct {
	profile *profile.Profile
	orders  []*order.Order
}

func (u *UserAggregator) Aggregate(parentCxt context.Context, id int) (*AggregatedResult, error) {
	u.logger.Info("Starting aggregate user ...")
	result := &AggregatedResult{}
	ctx, _ := context.WithTimeout(parentCxt, u.timeout)
	g, ctx := errgroup.WithContext(ctx)

	errGroupWrapper := func(caller func(ctx context.Context, id int) error) {
		g.Go(func() error {
			return caller(ctx, id)
		})
	}

	profileChan := u.fetchProfile(errGroupWrapper)
	ordersChan := u.fetchOrders(errGroupWrapper)

	// aggregate result
	g.Go(func() error {
		profileDone, ordersDone := false, false
		for !profileDone || !ordersDone {
			select {
			case profile, ok := <-profileChan:
				if ok {
					result.profile = profile
					profileDone = true
				}
			case orders, ok := <-ordersChan:
				if ok {
					result.orders = orders
					ordersDone = true
				}
			case <-ctx.Done():
				return ErrTimeout
			}
		}
		return nil
	})

	// waiting for result to be aggregated
	err := g.Wait()
	if err != nil {
		u.logger.Error("Failed on aggregating user")
		return nil, err
	}

	return result, nil
}

func (u *UserAggregator) fetchProfile(asyncWrapper func(func(ctx context.Context, id int) error)) <-chan *profile.Profile {
	out := make(chan *profile.Profile)
	asyncWrapper(func(ctx context.Context, id int) error {
		profile, err := u.profileService.Get(ctx, id)
		if err != nil {
			return err
		}
		out <- profile
		close(out)
		return nil
	})
	return out
}

func (u *UserAggregator) fetchOrders(asyncWrapper func(func(ctx context.Context, id int) error)) <-chan []*order.Order {
	out := make(chan []*order.Order)
	asyncWrapper(func(ctx context.Context, id int) error {
		orders, err := u.orderService.GetAll(ctx, id)
		if err != nil {
			return err
		}
		out <- orders
		close(out)
		return nil
	})
	return out
}
