package model

import "strconv"

type SendEmailUpdateStatusEvent struct {
	Email   string `json:"email"`
	Message string `json:"message"`
	UserID  int64  `json:"userID"`
}

func (u *SendEmailUpdateStatusEvent) GetId() string {
	return strconv.Itoa(int(u.UserID))
}
