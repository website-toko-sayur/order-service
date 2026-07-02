package messageproducer

import (
	"order-service/config"
	"order-service/internal/core/domain/model"

	"github.com/IBM/sarama"
)

type OrderPublishProducer struct {
	Producer[*model.OrderPublishEvent]
}

func NewOrderPublishProducer(producer sarama.SyncProducer, cfg *config.Config) *OrderPublishProducer {
	return &OrderPublishProducer{
		Producer: Producer[*model.OrderPublishEvent]{
			Producer: producer,
			Topic:    cfg.Topic.OrderPublish,
		},
	}
}
