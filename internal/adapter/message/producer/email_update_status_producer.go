package messageproducer

import (
	"order-service/config"
	"order-service/internal/core/domain/model"

	"github.com/IBM/sarama"
)

type EmailUpdateStatusProducer struct {
	Producer[*model.SendEmailUpdateStatusEvent]
}

func NewEmailUpdateStatusProducer(producer sarama.SyncProducer, cfg *config.Config) *EmailUpdateStatusProducer {
	return &EmailUpdateStatusProducer{
		Producer: Producer[*model.SendEmailUpdateStatusEvent]{
			Producer: producer,
			Topic:    cfg.Topic.EmailUpdateStatus,
		},
	}
}
