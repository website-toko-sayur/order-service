package model

import "strconv"

type UpdateStatusEvent struct {
	OrderID int64  `json:"orderID"`
	Status  string `json:"status"`
}

func (u *UpdateStatusEvent) GetId() string {
	return strconv.Itoa(int(u.OrderID))
}
