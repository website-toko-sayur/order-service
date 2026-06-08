package repository

import (
	"context"
	"errors"
	"math"
	"order-service/internal/core/domain/entity"
	"order-service/internal/core/domain/model"
	"time"

	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

type orderRepository struct {
	db *gorm.DB
}

type OrderRepositoryInterface interface {
	GetAll(ctx context.Context, queryString entity.QueryStringEntity) ([]entity.OrderEntity, int64, int64, error)
	GetByID(ctx context.Context, orderID int64) (*entity.OrderEntity, error)
	CreateOrder(ctx context.Context, req entity.OrderEntity) (int64, error)
	UpdateStatus(ctx context.Context, req entity.OrderEntity) (int64, string, string, error)
	DeleteOrder(ctx context.Context, orderID int64) error
	GetOrderByOrderCode(ctx context.Context, orderCode string) (*entity.OrderEntity, error)
}

func NewOrderRepository(db *gorm.DB) OrderRepositoryInterface {
	return &orderRepository{db: db}
}

func (o *orderRepository) GetOrderByOrderCode(ctx context.Context, orderCode string) (*entity.OrderEntity, error) {
	modelOrder := model.Order{}

	result := o.db.Preload("OrderItems").Where("order_code =?", orderCode).First(&modelOrder)
	if result.Error != nil {
		log.Error().
			Err(result.Error).
			Str("source", "internal.adapter.orderRepository.GetOrderByOrderCode")
		return nil, result.Error
	}

	if result.RowsAffected == 0 {
		log.Info().
			Str("source", "internal.adapter.orderRepository.GetOrderByOrderCode").
			Msg("order not found")
		return nil, errors.New("404")
	}

	orderItemEntities := []entity.OrderItemEntity{}
	for _, item := range modelOrder.OrderItems {
		orderItemEntities = append(orderItemEntities, entity.OrderItemEntity{
			ID:        item.ID,
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
		})
	}

	return &entity.OrderEntity{
		ID:           modelOrder.ID,
		OrderCode:    modelOrder.OrderCode,
		Status:       modelOrder.Status,
		BuyerId:      modelOrder.BuyerId,
		OrderDate:    modelOrder.OrderDate.Format("2006-01-02 15:04:05"),
		TotalAmount:  int64(modelOrder.TotalAmount),
		OrderItems:   orderItemEntities,
		Remarks:      modelOrder.Remarks,
		ShippingType: modelOrder.ShippingType,
		ShippingFee:  int64(modelOrder.ShippingFee),
	}, nil
}

func (o *orderRepository) CreateOrder(ctx context.Context, req entity.OrderEntity) (int64, error) {
	orderDate, err := time.Parse("2006-01-02", req.OrderDate) // YYYY-MM-DD
	if err != nil {
		log.Error().
			Err(err).
			Str("source", "internal.adapter.orderRepository.CreateOrder")
		return 0, err
	}

	var orderItems []model.OrderItem
	for _, item := range req.OrderItems {
		orderItem := model.OrderItem{
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
		}
		orderItems = append(orderItems, orderItem)
	}

	modelOrder := model.Order{
		OrderCode:    req.OrderCode,
		BuyerId:      req.BuyerId,
		OrderDate:    orderDate,
		OrderTime:    req.OrderTime,
		Status:       req.Status,
		TotalAmount:  float64(req.TotalAmount),
		ShippingType: req.ShippingType,
		ShippingFee:  float64(req.ShippingFee),
		Remarks:      req.Remarks,
		OrderItems:   orderItems,
	}

	if err := o.db.Create(&modelOrder).Error; err != nil {
		log.Error().
			Err(err).
			Str("source", "internal.adapter.orderRepository.CreateOrder").
			Msg("failed create order")
		return 0, err
	}

	return modelOrder.ID, nil
}

func (o *orderRepository) DeleteOrder(ctx context.Context, orderID int64) error {
	modelOrder := model.Order{}

	result := o.db.Preload("OrderItems").Where("id = ?", orderID).First(&modelOrder)
	if result.Error != nil {
		log.Error().
			Err(result.Error).
			Str("source", "internal.adapter.orderRepository.DeleteOrder")
		return result.Error
	}

	if result.RowsAffected == 0 {
		log.Info().
			Str("source", "internal.adapter.orderRepository.DeleteOrder").
			Msg("order not found")
		return errors.New("404")
	}

	if err := o.db.Select("OrderItems").Delete(&modelOrder).Error; err != nil {
		log.Error().
			Err(result.Error).
			Str("source", "internal.adapter.orderRepository.DeleteOrder")
		return err
	}

	return nil
}

func (o *orderRepository) UpdateStatus(ctx context.Context, req entity.OrderEntity) (int64, string, string, error) {
	modelOrder := model.Order{}

	result := o.db.Select("id", "order_code", "status", "buyer_id", "remarks").Where("id = ?", req.ID).First(&modelOrder)
	if result.Error != nil {
		log.Error().
			Err(result.Error).
			Str("source", "internal.adapter.orderRepository.UpdateStatus")
		return 0, "", "", result.Error
	}

	if result.RowsAffected == 0 {
		log.Info().
			Str("source", "internal.adapter.orderRepository.UpdateStatus").
			Msg("order not found")
		return 0, "", "", errors.New("404")
	}

	// key -> status saat ini; value -> status yang boleh dituju
	allowedTransitions := map[string][]string{
		"Pending":   {"Confirmed", "Cancelled"},
		"Confirmed": {"Process", "Cancelled"},
		"Process":   {"Sending", "Cancelled"},
		"Sending":   {"Done"},
		"Done":      {},
		"Cancelled": {},
	}

	allowed := false

	/*
		 allowed := false

		for _, status := range []string{"Confirmed", "Cancelled"} {
				if status == "Confirmed" {
					allowed = true
					break
				}
		}
	*/
	for _, status := range allowedTransitions[modelOrder.Status] {
		if status == req.Status {
			allowed = true
			break
		}
	}

	if !allowed {
		log.Warn().
			Str("source", "internal.adapter.orderRepository.UpdateStatus").
			Str("current_status", modelOrder.Status).
			Str("new_status", req.Status).
			Msg("invalid status transition")

		return 0, "", "", errors.New("400")
	}

	modelOrder.Status = req.Status
	modelOrder.Remarks = req.Remarks

	if err := o.db.UpdateColumns(&modelOrder).Error; err != nil {
		log.Error().
			Err(err).
			Str("source", "internal.adapter.orderRepository.UpdateStatus").
			Msg("failed update order status")
		return 0, "", "", err
	}

	return modelOrder.BuyerId, modelOrder.Status, modelOrder.OrderCode, nil
}

func (o *orderRepository) GetAll(ctx context.Context, queryString entity.QueryStringEntity) ([]entity.OrderEntity, int64, int64, error) {
	var modelOrders []model.Order
	var countData int64
	offset := (queryString.Page - 1) * queryString.Limit

	sqlMain := o.db.Preload("OrderItems").
		Where("order_code ILIKE ? OR status ILIKE ?", "%"+queryString.Search+"%", "%"+queryString.Status+"%")

	if queryString.BuyerID != 0 {
		sqlMain = sqlMain.Where("buyer_id = ?", queryString.BuyerID)
	}

	if err := sqlMain.Model(&modelOrders).Count(&countData).Error; err != nil {
		log.Error().
			Err(err).
			Str("source", "internal.adapter.orderRepository.GetAll")
		return nil, 0, 0, err
	}

	totalPage := int(math.Ceil(float64(countData) / float64(queryString.Limit)))
	if err := sqlMain.Order("order_date DESC").Limit(int(queryString.Limit)).Offset(int(offset)).Find(&modelOrders).Error; err != nil {
		log.Error().
			Err(err).
			Str("source", "internal.adapter.orderRepository.GetAll")
		return nil, 0, 0, err
	}

	if len(modelOrders) < 1 {
		err := errors.New("404")
		log.Info().
			Str("source", "internal.adapter.userRepository.GetCustomerAll").
			Msg("No customer found")
		return nil, 0, 0, err
	}

	entities := []entity.OrderEntity{}
	for _, val := range modelOrders {
		orderItemEntities := []entity.OrderItemEntity{}
		for _, item := range val.OrderItems {
			orderItemEntities = append(orderItemEntities, entity.OrderItemEntity{
				ID:        item.ID,
				ProductID: item.ProductID,
				Quantity:  item.Quantity,
			})
		}
		entities = append(entities, entity.OrderEntity{
			ID:          val.ID,
			OrderCode:   val.OrderCode,
			Status:      val.Status,
			OrderDate:   val.OrderDate.Format("2006-01-02"),
			OrderTime:   val.OrderTime,
			TotalAmount: int64(val.TotalAmount),
			OrderItems:  orderItemEntities,
			BuyerId:     val.BuyerId,
		})
	}

	return entities, countData, int64(totalPage), nil
}

func (o *orderRepository) GetByID(ctx context.Context, orderID int64) (*entity.OrderEntity, error) {
	var modelOrder model.Order

	if err := o.db.Preload("OrderItems").Where("id = ?", orderID).First(&modelOrder).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Info().
				Str("source", "internal.adapter.orderRepository.GetByID").
				Int64("order_id", orderID).
				Msg("order not found")

			return nil, errors.New("404")
		}

		log.Error().
			Err(err).
			Str("source", "internal.adapter.orderRepository.GetByID").
			Int64("order_id", orderID).
			Msg("failed get order by id")

		return nil, errors.New("500")
	}

	orderItemEntities := make([]entity.OrderItemEntity, 0, len(modelOrder.OrderItems))

	for _, item := range modelOrder.OrderItems {
		orderItemEntities = append(orderItemEntities, entity.OrderItemEntity{
			ID:        item.ID,
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
		})
	}

	return &entity.OrderEntity{
		ID:           modelOrder.ID,
		OrderCode:    modelOrder.OrderCode,
		Status:       modelOrder.Status,
		BuyerId:      modelOrder.BuyerId,
		OrderDate:    modelOrder.OrderDate.Format("2006-01-02 15:04:05"),
		TotalAmount:  int64(modelOrder.TotalAmount),
		OrderItems:   orderItemEntities,
		Remarks:      modelOrder.Remarks,
		ShippingType: modelOrder.ShippingType,
		ShippingFee:  int64(modelOrder.ShippingFee),
	}, nil
}
