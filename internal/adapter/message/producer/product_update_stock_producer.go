package messageproducer

import (
	"order-service/config"
	"order-service/internal/core/domain/model"

	"github.com/IBM/sarama"
)

type UpdateStockProducer struct {
	Producer[*model.ProductUpdateStockEvent]
}

func NewUpdateStockProducer(producer sarama.SyncProducer, cfg *config.Config) *UpdateStockProducer {
	return &UpdateStockProducer{
		Producer: Producer[*model.ProductUpdateStockEvent]{
			Producer: producer,
			Topic:    cfg.Topic.ProductUpdateStock,
		},
	}
}
