package model

import "strconv"

type SendEmailUpdateStatusEvent struct {
	ReceiverEmail    string `json:"receiver_email"`
	Message          string `json:"message"`
	ReceiverID       int64  `json:"receiver_id"`
	Subject          string `json:"subject"`
	NotificationType string `json:"notification_type"`
}

func (u *SendEmailUpdateStatusEvent) GetId() string {
	return strconv.Itoa(int(u.ReceiverID))
}
