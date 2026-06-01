package messageproducer

import (
	"order-service/config"
	"order-service/internal/core/domain/model"

	"github.com/IBM/sarama"
)

type NotifUpdateStatusProducer struct {
	Producer[*model.SendPushNotifOrderUpdateStatusEvent]
}

func NewNotifUpdateStatusProducer(producer sarama.SyncProducer, cfg *config.Config) *NotifUpdateStatusProducer {
	return &NotifUpdateStatusProducer{
		Producer: Producer[*model.SendPushNotifOrderUpdateStatusEvent]{
			Producer: producer,
			Topic:    cfg.Topic.PushNotifOrder,
		},
	}
}
