package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"order-service/config"
	"order-service/internal/adapter/httpclient"
	messageproducer "order-service/internal/adapter/message/producer"
	"order-service/internal/adapter/repository"
	"order-service/internal/core/domain/entity"
	"order-service/internal/core/domain/model"
	"order-service/utils/conv"
	"strconv"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type orderService struct {
	repo                                   repository.OrderRepositoryInterface
	elasticRepo                            repository.ElasticRepositoryInterface
	cfg                                    *config.Config
	httpClient                             httpclient.Client
	orderDeleteProducer                    *messageproducer.OrderDeleteProducer
	sendEmailUpdateStatusOrderProducer     *messageproducer.EmailUpdateStatusProducer
	sendpushNotifUpdateStatusOrderProducer *messageproducer.NotifUpdateStatusProducer
	updateStatusOrderProducer              *messageproducer.UpdateStatusProducer
	updateProductStockProducer             *messageproducer.UpdateStockProducer
	orderPublishProducer                   *messageproducer.OrderPublishProducer
}

type OrderServiceInterface interface {
	GetAll(ctx context.Context, queryString entity.QueryStringEntity, accessToken string) ([]entity.OrderEntity, int64, int64, error)
	GetByID(ctx context.Context, orderID int64, accessToken string) (*entity.OrderEntity, error)
	CreateOrder(ctx context.Context, req entity.OrderEntity, accessToken string) (int64, error)
	UpdateStatus(ctx context.Context, req entity.OrderEntity, accessToken string) error
	GetAllCustomer(ctx context.Context, queryString entity.QueryStringEntity, accessToken string) ([]entity.OrderEntity, int64, int64, error)
	GetDetailCustomer(ctx context.Context, orderID int64, accessToken string) (*entity.OrderEntity, error)
	DeleteByID(ctx context.Context, orderID int64) error
	GetOrderByOrderCode(ctx context.Context, orderCode, accessToken string) (*entity.OrderEntity, error)
	GetPublicOrderIDByOrderCode(ctx context.Context, orderCode string) (int64, error)
}

func NewOrderService(
	repo repository.OrderRepositoryInterface,
	elasticRepo repository.ElasticRepositoryInterface,
	cfg *config.Config,
	httpClient httpclient.Client,
	orderDeleteProducer *messageproducer.OrderDeleteProducer,
	sendEmailUpdateStatusOrderProducer *messageproducer.EmailUpdateStatusProducer,
	sendpushNotifUpdateStatusOrderProducer *messageproducer.NotifUpdateStatusProducer,
	updateStatusOrderProducer *messageproducer.UpdateStatusProducer,
	updateProductStockProducer *messageproducer.UpdateStockProducer,
	orderPublishProducer *messageproducer.OrderPublishProducer,
) OrderServiceInterface {
	return &orderService{
		repo:                                   repo,
		elasticRepo:                            elasticRepo,
		cfg:                                    cfg,
		httpClient:                             httpClient,
		orderDeleteProducer:                    orderDeleteProducer,
		sendEmailUpdateStatusOrderProducer:     sendEmailUpdateStatusOrderProducer,
		sendpushNotifUpdateStatusOrderProducer: sendpushNotifUpdateStatusOrderProducer,
		updateStatusOrderProducer:              updateStatusOrderProducer,
		updateProductStockProducer:             updateProductStockProducer,
		orderPublishProducer:                   orderPublishProducer,
	}
}

func (o *orderService) GetPublicOrderIDByOrderCode(ctx context.Context, orderCode string) (int64, error) {
	result, err := o.repo.GetOrderByOrderCode(ctx, orderCode)
	if err != nil {
		log.Error().
			Err(err).
			Str("source", "internal.core.orderService.GetPublicOrderIDByOrderCode")
		return 0, err
	}

	return result.ID, nil
}

func (o *orderService) GetOrderByOrderCode(ctx context.Context, orderCode string, accessToken string) (*entity.OrderEntity, error) {
	result, err := o.repo.GetOrderByOrderCode(ctx, orderCode)
	if err != nil {
		log.Error().
			Err(err).
			Str("source", "internal.core.orderService.GetOrderByOrderCode")
		return nil, err
	}

	requestID := uuid.NewString()

	userResponse, err := o.httpClientUserService(requestID, result.BuyerId)
	if err != nil {
		log.Error().
			Err(err).
			Str("source", "internal.core.orderService.GetOrderByOrderCode")
		return nil, err
	}

	result.BuyerName = userResponse.Name
	result.BuyerEmail = userResponse.Email
	result.BuyerPhone = userResponse.Phone
	result.BuyerAddress = userResponse.Address

	for key, val := range result.OrderItems {
		productResponse, err := o.httpClientProductService(requestID, val.ProductID)
		if err != nil {
			log.Error().
				Err(err).
				Str("source", "internal.core.orderService.GetOrderByOrderCode")
			return nil, err
		}

		result.OrderItems[key].ProductImage = productResponse.Image
		result.OrderItems[key].ProductName = productResponse.Name
		result.OrderItems[key].Price = int64(productResponse.SalePrice)
	}

	return result, nil
}

func (o *orderService) DeleteByID(ctx context.Context, orderID int64) error {
	err := o.repo.DeleteOrder(ctx, orderID)
	if err != nil {
		log.Error().
			Err(err).
			Str("source", "internal.core.orderService.DeleteByID")
		return err
	}

	if o.orderDeleteProducer != nil {
		event := &model.OrderDeleteEvent{
			OrderID: orderID,
		}

		log.Info().
			Str("source", "internal.core.orderService.DeleteByID").
			Msg("Publishing order delete event")

		if err = o.orderDeleteProducer.Send(event); err != nil {
			log.Warn().
				Err(err).
				Str("source", "internal.core.orderService.DeleteByID").
				Msg("Failed publish order delete event")
			return err
		}
	} else {
		log.Info().
			Str("source", "internal.core.orderService.DeleteByID").
			Msg("Kafka producer is disabled, skipping order delete event")
	}

	return nil
}

func (o *orderService) GetDetailCustomer(ctx context.Context, orderID int64, accessToken string) (*entity.OrderEntity, error) {
	result, err := o.repo.GetByID(ctx, orderID)
	if err != nil {
		log.Error().
			Err(err).
			Str("source", "internal.core.orderService.GetDetailCustomer")
		return nil, err
	}

	requestID := uuid.NewString()

	userResponse, err := o.httpClientUserService(requestID, result.BuyerId)
	if err != nil {
		log.Error().
			Err(err).
			Str("source", "internal.core.orderService.GetDetailCustomer")
		return nil, err
	}

	result.BuyerName = userResponse.Name
	result.BuyerEmail = userResponse.Email
	result.BuyerPhone = userResponse.Phone
	result.BuyerAddress = userResponse.Address

	for key, val := range result.OrderItems {
		productResponse, err := o.httpClientProductService(requestID, val.ProductID)
		if err != nil {
			log.Error().
				Err(err).
				Str("source", "internal.core.orderService.GetDetailCustomer")
			return nil, err
		}

		result.OrderItems[key].ProductImage = productResponse.Image
		if productResponse.Child != nil {
			result.OrderItems[key].ProductImage = productResponse.Child[0].Image
		}
		result.OrderItems[key].ProductName = productResponse.Name
		result.OrderItems[key].Price = int64(productResponse.SalePrice)
		result.OrderItems[key].ProductWeight = int64(productResponse.Weight)
		result.OrderItems[key].ProductUnit = productResponse.Unit
	}

	return result, nil
}

func (o *orderService) GetAllCustomer(ctx context.Context, queryString entity.QueryStringEntity, accessToken string) ([]entity.OrderEntity, int64, int64, error) {
	results, count, total, err := o.elasticRepo.SearchOrderElasticByBuyerId(ctx, queryString, queryString.BuyerID)
	if err == nil {
		return results, count, total, nil
	} else {
		log.Error().
			Err(err).
			Str("source", "internal.core.orderService.GetAllCustomer")
	}

	results, count, total, err = o.repo.GetAll(ctx, queryString)
	if err != nil {
		log.Error().
			Err(err).
			Str("source", "internal.core.orderService.GetAllCustomer")
		return nil, 0, 0, err
	}

	requestID := uuid.NewString()

	for key, val := range results {
		userResponse, err := o.httpClientUserService(requestID, val.BuyerId)
		if err != nil {
			log.Error().
				Err(err).
				Str("source", "internal.core.orderService.GetAllCustomer")
			return nil, 0, 0, err
		}

		results[key].BuyerName = userResponse.Name
		results[key].BuyerEmail = userResponse.Email
		results[key].BuyerPhone = userResponse.Phone
		results[key].BuyerAddress = userResponse.Address

		for key2, res := range val.OrderItems {

			productResponse, err := o.httpClientProductService(requestID, res.ProductID)
			if err != nil {
				log.Error().
					Err(err).
					Str("source", "internal.core.orderService.GetAllCustomer")
				return nil, 0, 0, err
			}

			val.OrderItems[key2].ProductImage = productResponse.Image
			val.OrderItems[key2].ProductName = productResponse.Name
			val.OrderItems[key2].Price = int64(productResponse.SalePrice)
			val.OrderItems[key2].Quantity = res.Quantity
			val.OrderItems[key2].ProductUnit = productResponse.Unit
			val.OrderItems[key2].ProductWeight = int64(productResponse.Weight)
		}
	}

	return results, count, total, nil
}

func (o *orderService) UpdateStatus(ctx context.Context, req entity.OrderEntity, accessToken string) error {
	buyerID, statusOrder, orderCode, err := o.repo.UpdateStatus(ctx, req)
	if err != nil {
		log.Error().
			Err(err).
			Str("source", "internal.core.orderService.UpdateStatus")
		return err
	}

	requestID := uuid.NewString()

	userResponse, err := o.httpClientUserService(requestID, buyerID)
	if err != nil {
		log.Error().
			Err(err).
			Str("source", "internal.core.orderService.UpdateStatus")
		return err
	}

	message := fmt.Sprintf("Hello,\n\nYour order with ID %s has been updated to status: %s.\n\nThank you for shopping with us!", orderCode, statusOrder)

	if o.sendEmailUpdateStatusOrderProducer != nil {
		event := &model.SendEmailUpdateStatusEvent{
			Email:   userResponse.Email,
			Message: message,
			UserID:  buyerID,
		}

		log.Info().
			Str("source", "internal.core.orderService.UpdateStatus").
			Msg("Publishing send email update status event")

		if err = o.sendEmailUpdateStatusOrderProducer.Send(event); err != nil {
			log.Warn().
				Err(err).
				Str("source", "internal.core.orderService.UpdateStatus").
				Msg("Failed Publish send email update status event")
			return err
		}
	} else {
		log.Info().
			Str("source", "internal.core.orderService.UpdateStatus").
			Msg("Kafka producer is disabled, skipping send email update status event")
	}

	if o.sendpushNotifUpdateStatusOrderProducer != nil {
		event := &model.SendPushNotifOrderUpdateStatusEvent{
			Message: message,
			UserID:  buyerID,
		}

		log.Info().
			Str("source", "internal.core.orderService.UpdateStatus").
			Msg("Publishing send push notif order update status event")

		if err = o.sendpushNotifUpdateStatusOrderProducer.Send(event); err != nil {
			log.Warn().
				Err(err).
				Str("source", "internal.core.orderService.UpdateStatus").
				Msg("Failed publish send push notif order update status event")
			return err
		}
	} else {
		log.Info().
			Str("source", "internal.core.orderService.UpdateStatus").
			Msg("Kafka producer is disabled, skipping send push notif order update status event")
	}

	if o.updateStatusOrderProducer != nil {
		event := &model.UpdateStatusEvent{
			OrderID: req.ID,
			Status:  req.Status,
		}

		log.Info().
			Str("source", "internal.core.orderService.UpdateStatus").
			Msg("Publishing update status order event")

		if err = o.updateStatusOrderProducer.Send(event); err != nil {
			log.Warn().
				Err(err).
				Str("source", "internal.core.orderService.UpdateStatus").
				Msg("Failed publish update status order event")
			return err
		}
	} else {
		log.Info().
			Str("source", "internal.core.orderService.UpdateStatus").
			Msg("Kafka producer is disabled, skipping update status order event")
	}

	return nil
}

func (o *orderService) CreateOrder(ctx context.Context, req entity.OrderEntity, accessToken string) (int64, error) {
	req.OrderCode = conv.GenerateOrderCode()
	shippingFee := 0
	if req.ShippingType == "Delivery" {
		shippingFee = 5000
	}
	req.ShippingFee = int64(shippingFee)
	req.Status = "Pending"
	orderID, err := o.repo.CreateOrder(ctx, req)
	if err != nil {
		log.Error().
			Err(err).
			Str("source", "internal.core.orderService.CreateOrder")
		return 0, err
	}

	resultData, err := o.GetByID(ctx, orderID, accessToken)
	if err != nil {
		log.Error().
			Err(err).
			Str("source", "internal.core.orderService.CreateOrder")
	}

	if o.orderPublishProducer != nil {
		orderItems := make([]model.OrderItemEvent, 0, len(resultData.OrderItems))

		for _, item := range resultData.OrderItems {
			orderItems = append(orderItems, model.OrderItemEvent{
				ID:            item.ID,
				OrderID:       item.OrderID,
				ProductID:     item.ProductID,
				Quantity:      item.Quantity,
				OrderCode:     item.OrderCode,
				ProductName:   item.ProductName,
				ProductImage:  item.ProductImage,
				Price:         item.Price,
				ProductUnit:   item.ProductUnit,
				ProductWeight: item.ProductWeight,
			})
		}

		event := &model.OrderPublishEvent{
			ID:            resultData.ID,
			OrderCode:     resultData.OrderCode,
			BuyerId:       resultData.BuyerId,
			OrderDate:     resultData.OrderDate,
			Status:        resultData.Status,
			TotalAmount:   resultData.TotalAmount,
			PaymentMethod: resultData.PaymentMethod,
			ShippingType:  resultData.ShippingType,
			ShippingFee:   resultData.ShippingFee,
			OrderTime:     resultData.OrderTime,
			Remarks:       resultData.Remarks,
			CreatedAt:     resultData.CreatedAt,
			OrderItems:    orderItems,
			BuyerName:     resultData.BuyerName,
			BuyerEmail:    resultData.BuyerEmail,
			BuyerPhone:    resultData.BuyerPhone,
			BuyerAddress:  resultData.BuyerAddress,
			BuyerLat:      resultData.BuyerLat,
			BuyerLng:      resultData.BuyerLng,
		}

		log.Info().
			Str("source", "internal.core.orderService.CreateOrder").
			Msg("Publishing order publish event")

		if err = o.orderPublishProducer.Send(event); err != nil {
			log.Warn().
				Err(err).
				Str("source", "internal.core.orderService.CreateOrder").
				Msg("Failed publish product publish event")
			return 0, err
		}
	}

	for _, orderItem := range req.OrderItems {
		if o.updateProductStockProducer != nil {
			event := &model.ProductUpdateStockEvent{
				ProductID: orderItem.ProductID,
				Quantity:  orderItem.Quantity,
			}

			log.Info().
				Str("source", "internal.core.orderService.CreateOrder").
				Msg("Publishing update product stock event")

			if err = o.updateProductStockProducer.Send(event); err != nil {
				log.Warn().
					Err(err).
					Str("source", "internal.core.orderService.CreateOrder").
					Msg("Failed publish update product stock event")
				return 0, err
			}
		} else {
			log.Info().
				Str("source", "internal.core.orderService.CreateOrder").
				Msg("Kafka producer is disabled, skipping update product stock event")
		}
	}

	return orderID, nil
}

func (o *orderService) GetByID(ctx context.Context, orderID int64, accessToken string) (*entity.OrderEntity, error) {
	result, err := o.repo.GetByID(ctx, orderID)
	if err != nil {
		log.Error().
			Err(err).
			Str("source", "internal.core.service.orderService.GetByID")
		return nil, err
	}

	requestID := uuid.NewString()

	userResponse, err := o.httpClientUserService(requestID, result.BuyerId)
	if err != nil {
		log.Error().
			Err(err).
			Str("source", "internal.core.service.orderService.GetByID")
		return nil, err
	}

	result.BuyerName = userResponse.Name
	result.BuyerEmail = userResponse.Email
	result.BuyerPhone = userResponse.Phone
	result.BuyerAddress = userResponse.Address

	for key, val := range result.OrderItems {
		productResponse, err := o.httpClientProductService(requestID, val.ProductID)
		if err != nil {
			log.Error().
				Err(err).
				Str("source", "internal.core.service.orderService.GetByID")
			return nil, err
		}

		result.OrderItems[key].ProductImage = productResponse.Image
		result.OrderItems[key].ProductName = productResponse.Name
		result.OrderItems[key].Price = int64(productResponse.SalePrice)
	}

	return result, nil
}

func (o *orderService) GetAll(ctx context.Context, queryString entity.QueryStringEntity, accessToken string) ([]entity.OrderEntity, int64, int64, error) {
	results, count, total, err := o.elasticRepo.SearchOrderElastic(ctx, queryString)
	if err == nil {
		return results, count, total, nil
	} else {
		log.Error().
			Err(err).
			Str("source", "internal.core.service.orderService.GetAll")
	}

	results, count, total, err = o.repo.GetAll(ctx, queryString)
	if err != nil {
		log.Error().
			Err(err).
			Str("source", "internal.core.service.orderService.GetAll")
		return nil, 0, 0, err
	}

	requestID := uuid.NewString()

	for key, val := range results {

		userResponse, err := o.httpClientUserService(requestID, val.BuyerId)
		if err != nil {
			log.Error().
				Err(err).
				Str("source", "internal.core.service.orderService.GetAll")
			return nil, 0, 0, err
		}
		results[key].BuyerName = userResponse.Name

		for key2, res := range val.OrderItems {

			productResponse, err := o.httpClientProductService(requestID, res.ProductID)
			if err != nil {
				log.Error().
					Err(err).
					Str("source", "internal.core.service.orderService.GetAll")
				return nil, 0, 0, err
			}

			val.OrderItems[key2].ProductImage = productResponse.Image
		}
	}

	return results, count, total, nil
}

func (o *orderService) httpClientUserService(requestID string, userID int64) (*entity.CustomerResponse, error) {
	baseUrlUser := fmt.Sprintf("%s/%s", o.cfg.App.UserServiceUrl, "internal/users/"+strconv.FormatInt(userID, 10))
	header := map[string]string{
		"X-Internal-Service": "true",
		"X-Internal-Secret":  o.cfg.App.InternalSecretKey,
		"X-From-Service":     "order-service",
		"Accept":             "application/json",
		"X-Request-ID":       requestID,
	}
	dataUser, err := o.httpClient.CallURL("GET", baseUrlUser, header, nil)
	if err != nil {
		log.Error().
			Err(err).
			Str("request_id", requestID).
			Str("source", "internal.core.orderService.httpClientUserService").
			Msg("failed request user service")
		return nil, err
	}

	defer dataUser.Body.Close()

	body, err := io.ReadAll(dataUser.Body)
	if err != nil {
		log.Error().
			Err(err).
			Str("request_id", requestID).
			Str("source", "internal.core.orderService.httpClientUserService").
			Msg("failed read user service response")
		return nil, err
	}

	var userResponse entity.UserHttpClientResponse
	err = json.Unmarshal(body, &userResponse)
	if err != nil {
		log.Error().
			Err(err).
			Str("request_id", requestID).
			Str("source", "internal.core.orderService.httpClientUserService").
			Msg("failed unmarshal user response")
		return nil, err
	}

	return &userResponse.Data, nil
}

func (o *orderService) httpClientProductService(requestID string, productID int64) (*entity.ProductResponseEntity, error) {
	baseUrlUser := fmt.Sprintf("%s/%s", o.cfg.App.UserServiceUrl, "internal/products/"+strconv.FormatInt(productID, 10))
	header := map[string]string{
		"X-Internal-Service": "true",
		"X-Internal-Secret":  o.cfg.App.InternalSecretKey,
		"X-From-Service":     "order-service",
		"Accept":             "application/json",
		"X-Request-ID":       requestID,
	}
	dataUser, err := o.httpClient.CallURL("GET", baseUrlUser, header, nil)
	if err != nil {
		log.Error().
			Err(err).
			Str("request_id", requestID).
			Str("source", "internal.core.orderService.httpClientProductService").
			Msg("failed request product service")
		return nil, err
	}

	defer dataUser.Body.Close()

	body, err := io.ReadAll(dataUser.Body)
	if err != nil {
		log.Error().
			Err(err).
			Str("request_id", requestID).
			Str("source", "internal.core.orderService.httpClientProductService").
			Msg("failed read product service response")
		return nil, err
	}

	var productResponse entity.ProductHttpClientResponse
	err = json.Unmarshal(body, &productResponse)
	if err != nil {
		log.Error().
			Err(err).
			Str("request_id", requestID).
			Str("source", "internal.core.orderService.httpClientProductService").
			Msg("failed unmarshal product response")
		return nil, err
	}

	return &productResponse.Data, nil
}
