package model

import "strconv"

type SendPushNotifOrderUpdateStatusEvent struct {
	Message string `json:"message"`
	UserID  int64  `json:"userID"`
}

func (u *SendPushNotifOrderUpdateStatusEvent) GetId() string {
	return strconv.Itoa(int(u.UserID))
}
