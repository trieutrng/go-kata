package order

import "context"

type Order struct {
	Id     int `json:"id"`
	UserId int `json:"user_id"`
}

type Service interface {
	GetAll(ctx context.Context, userId int) ([]*Order, error)
}
