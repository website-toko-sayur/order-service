package messageproducer

import (
	"order-service/config"
	"order-service/internal/core/domain/model"

	"github.com/IBM/sarama"
)

type UpdateStatusProducer struct {
	Producer[*model.UpdateStatusEvent]
}

func NewUpdateStatusProducer(producer sarama.SyncProducer, cfg *config.Config) *UpdateStatusProducer {
	return &UpdateStatusProducer{
		Producer: Producer[*model.UpdateStatusEvent]{
			Producer: producer,
			Topic:    cfg.Topic.PublisherUpdateStatus,
		},
	}
}
