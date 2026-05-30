package model

import "strconv"

type OrderDeleteEvent struct {
	OrderID int64 `json:"orderID"`
}

func (u *OrderDeleteEvent) GetId() string {
	return strconv.Itoa(int(u.OrderID))
}
