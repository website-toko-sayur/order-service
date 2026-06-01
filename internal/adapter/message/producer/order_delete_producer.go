package messageproducer

import (
	"order-service/config"
	"order-service/internal/core/domain/model"

	"github.com/IBM/sarama"
)

type OrderDeleteProducer struct {
	Producer[*model.OrderDeleteEvent]
}

func NewOrderDeleteProducer(producer sarama.SyncProducer, cfg *config.Config) *OrderDeleteProducer {
	return &OrderDeleteProducer{
		Producer: Producer[*model.OrderDeleteEvent]{
			Producer: producer,
			Topic:    cfg.Topic.PublisherDeleteOrder,
		},
	}
}
