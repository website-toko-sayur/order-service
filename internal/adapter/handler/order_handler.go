package handler

import (
	"encoding/json"
	"fmt"
	"order-service/config"
	"order-service/internal/adapter"
	"order-service/internal/adapter/handler/request"
	"order-service/internal/adapter/handler/response"
	"order-service/internal/core/domain/entity"
	"order-service/internal/core/service"
	"order-service/internal/middleware"
	"order-service/utils/conv"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/gofiber/fiber/v3"
)

type orderHandler struct {
	orderService service.OrderServiceInterface
}

type OrderHandlerInterface interface {
	GetAllAdmin(c fiber.Ctx) error
	GetByIDAdmin(c fiber.Ctx) error
	CreateOrder(c fiber.Ctx) error
	UpdateStatus(c fiber.Ctx) error
	GetAllCustomer(c fiber.Ctx) error
	GetDetailCustomer(c fiber.Ctx) error
	DeleteByID(c fiber.Ctx) error
	GetOrderByOrderCode(c fiber.Ctx) error
	GetPublicOrderByOrderCode(c fiber.Ctx) error
}

func NewOrderHandler(
	app *fiber.App,
	orderService service.OrderServiceInterface,
	cfg *config.Config,
	jwtService service.JwtServiceInterface,
	redis *redis.Client,
) OrderHandlerInterface {
	orderHandler := &orderHandler{
		orderService: orderService,
	}

	mid := adapter.NewMiddlewareAdapter(cfg, jwtService, redis)
	midGateway := middleware.GatewayValidationMiddleware(cfg)
	midInternal := middleware.InternalValidationMiddleware(cfg)

	// public route via gateway
	app.Get("public/orders/:orderCode/code", midGateway, orderHandler.GetPublicOrderByOrderCode)

	// auth route via gateway
	authGroup := app.Group("auth", midGateway, mid.CheckToken())
	authGroup.Post("/orders", orderHandler.CreateOrder, mid.DistanceCheck())
	authGroup.Get("/orders", orderHandler.GetAllCustomer)
	authGroup.Get("/orders/:orderID", orderHandler.GetDetailCustomer)
	authGroup.Get("/orders/:orderCode/code", orderHandler.GetOrderByOrderCode)

	// admin route via gateway
	adminGroup := app.Group("/admin", midGateway, mid.CheckToken())
	adminGroup.Get("/orders", orderHandler.GetAllAdmin)
	adminGroup.Get("/orders/:orderID", orderHandler.GetByIDAdmin)
	adminGroup.Put("/orders/:orderID/status", orderHandler.UpdateStatus)
	adminGroup.Delete("/orders/:orderID", orderHandler.DeleteByID)

	// internal route
	internalGroup := app.Group("/internal", midInternal)
	internalGroup.Get("/orders/:orderCode/code", orderHandler.GetOrderByOrderCodeInternal)
	internalGroup.Get("/public/orders/:orderCode/code", orderHandler.GetPublicOrderByOrderCode)
	internalGroup.Get("/orders/:orderID", orderHandler.GetDetailCustomer)

	return orderHandler
}

func (o *orderHandler) GetPublicOrderByOrderCode(c fiber.Ctx) error {
	ctx := c.Context()

	orderCode := c.Params("orderCode")
	if orderCode == "" {
		log.Info().
			Str("source", "internal.adapter.orderHandler.GetPublicOrderByOrderCode").
			Msg("missing or invalid order code")
		return fiber.NewError(fiber.StatusBadRequest, "missing or invalid order code")
	}

	order, err := o.orderService.GetPublicOrderIDByOrderCode(ctx, orderCode)
	if err != nil {
		log.Error().
			Err(err).
			Str("orderCode", orderCode).
			Str("source", "internal.adapter.orderHandler.GetPublicOrderByOrderCode").
			Msg("failed get order ID")
		if err.Error() == "404" {
			return fiber.NewError(fiber.StatusNotFound, "order ID not found")
		}
		return err
	}

	return c.Status(fiber.StatusOK).JSON(response.DefaultResponse{
		Message: "success get order id",
		Data: map[string]int64{
			"orderID": order,
		},
	})
}

func (o *orderHandler) GetOrderByOrderCode(c fiber.Ctx) error {
	ctx := c.Context()

	orderCode := c.Params("orderCode")
	if orderCode == "" {
		log.Info().
			Str("source", "internal.adapter.orderHandler.GetOrderByOrderCode").
			Msg("missing or invalid order code")

		return fiber.NewError(fiber.StatusBadRequest, "missing or invalid order code")
	}

	order, err := o.orderService.GetOrderByOrderCode(ctx, orderCode)
	if err != nil {
		log.Error().
			Err(err).
			Str("order code", orderCode).
			Str("source", "internal.adapter.orderHandler.GetOrderByOrderCode").
			Msg("failed get order")

		if err.Error() == "404" {
			return fiber.NewError(fiber.StatusNotFound, "order not found")
		}

		return err
	}

	respOrder := response.OrderAdminDetail{
		ID:            order.ID,
		OrderCode:     order.OrderCode,
		ProductImage:  "",
		OrderDatetime: order.OrderDate,
		Status:        order.Status,
		PaymentMethod: order.PaymentMethod,
		ShippingFee:   order.ShippingFee,
		ShippingType:  order.ShippingType,
		Remarks:       order.Remarks,
		TotalAmount:   order.TotalAmount,
		Customer: response.CustomerOrder{
			CustomerName:    order.BuyerName,
			CustomerPhone:   order.BuyerPhone,
			CustomerAddress: order.BuyerAddress,
			CustomerEmail:   order.BuyerEmail,
			CustomerID:      order.BuyerId,
		},
		OrderDetail: make([]response.OrderDetail, 0, len(order.OrderItems)),
	}

	for _, item := range order.OrderItems {
		respOrder.OrderDetail = append(respOrder.OrderDetail, response.OrderDetail{
			ProductName:  item.ProductName,
			ProductImage: item.ProductImage,
			ProductPrice: item.Price,
			Quantity:     item.Quantity,
		})

		if respOrder.ProductImage == "" {
			respOrder.ProductImage = item.ProductImage
		}
	}

	return c.Status(fiber.StatusOK).JSON(response.DefaultResponse{
		Message: "success get order by order code",
		Data:    respOrder,
	})
}

func (o *orderHandler) DeleteByID(c fiber.Ctx) error {
	ctx := c.Context()

	user, ok := c.Locals("user").(string)
	if !ok || user == "" {
		log.Error().
			Str("source", "internal.adapter.orderHandler.DeleteByID").
			Msg("data token not found")

		return fiber.NewError(fiber.StatusUnauthorized, "data token not valid")
	}

	idParams := c.Params("orderID")
	if idParams == "" {
		log.Info().
			Str("source", "internal.adapter.orderHandler.DeleteByID").
			Msg("missing or invalid order ID")

		return fiber.NewError(fiber.StatusBadRequest, "missing or invalid order ID")
	}

	orderID, err := conv.StringToInt64(idParams)
	if err != nil {
		log.Info().
			Err(err).
			Str("order id", idParams).
			Str("source", "internal.adapter.orderHandler.DeleteByID").
			Msg("invalid order ID")

		return fiber.NewError(fiber.StatusBadRequest, "Invalid order ID")
	}

	err = o.orderService.DeleteByID(ctx, orderID)
	if err != nil {
		log.Error().
			Err(err).
			Int64("order id", orderID).
			Str("source", "internal.adapter.orderHandler.DeleteByID").
			Msg("failed delete order")

		if err.Error() == "404" {
			return fiber.NewError(fiber.StatusNotFound, "order not found")
		}

		return err
	}

	return c.Status(fiber.StatusOK).JSON(response.DefaultResponse{
		Message: "order deleted successfully",
		Data:    nil,
	})
}

func (o *orderHandler) GetDetailCustomer(c fiber.Ctx) error {
	ctx := c.Context()

	orderIDStr := c.Params("orderID")
	if orderIDStr == "" {
		log.Info().
			Str("source", "internal.adapter.orderHandler.GetDetailCustomer").
			Msg("missing or invalid order ID")

		return fiber.NewError(fiber.StatusBadRequest, "missing or invalid order ID")
	}

	orderID, err := conv.StringToInt64(orderIDStr)
	if err != nil {
		log.Info().
			Err(err).
			Str("order id", orderIDStr).
			Str("source", "internal.adapter.orderHandler.GetDetailCustomer").
			Msg("invalid order ID")

		return fiber.NewError(fiber.StatusBadRequest, "invalid order ID")
	}

	order, err := o.orderService.GetDetailCustomer(ctx, orderID)
	if err != nil {
		log.Error().
			Err(err).
			Int64("order id", orderID).
			Str("source", "internal.adapter.orderHandler.GetDetailCustomer").
			Msg("failed get order")

		if err.Error() == "404" {
			return fiber.NewError(fiber.StatusNotFound, "order not found")
		}

		return err
	}

	respOrder := response.OrderAdminDetail{
		ID:            order.ID,
		OrderCode:     order.OrderCode,
		ProductImage:  "",
		OrderDatetime: order.OrderDate,
		Status:        order.Status,
		PaymentMethod: order.PaymentMethod,
		ShippingFee:   order.ShippingFee,
		ShippingType:  order.ShippingType,
		Remarks:       order.Remarks,
		TotalAmount:   order.TotalAmount,
		Customer: response.CustomerOrder{
			CustomerName:    order.BuyerName,
			CustomerPhone:   order.BuyerPhone,
			CustomerAddress: order.BuyerAddress,
			CustomerEmail:   order.BuyerEmail,
			CustomerID:      order.BuyerId,
		},
		OrderDetail: make([]response.OrderDetail, 0, len(order.OrderItems)),
	}

	for _, item := range order.OrderItems {
		respOrder.OrderDetail = append(respOrder.OrderDetail, response.OrderDetail{
			ProductName:  item.ProductName,
			ProductImage: item.ProductImage,
			ProductPrice: item.Price,
			Quantity:     item.Quantity,
		})

		if respOrder.ProductImage == "" {
			respOrder.ProductImage = item.ProductImage
		}
	}

	return c.Status(fiber.StatusOK).JSON(response.DefaultResponse{
		Message: "success get order by order code",
		Data:    respOrder,
	})
}

func (o *orderHandler) GetAllCustomer(c fiber.Ctx) error {
	var (
		ctx         = c.Context()
		jwtUserData = entity.JwtUserData{}
	)

	user, ok := c.Locals("user").(string)
	if !ok || user == "" {
		log.Error().
			Str("source", "internal.adapter.orderHandler.GetAllCustomer").
			Msg("data token not found")

		return fiber.NewError(fiber.StatusUnauthorized, "data token not valid")
	}

	if err := json.Unmarshal([]byte(user), &jwtUserData); err != nil {
		log.Error().
			Err(err).
			Str("source", "internal.adapter.orderHandler.GetAllCustomer").
			Msg("failed parse jwt user data")

		return fiber.NewError(fiber.StatusBadRequest, "invalid token data")
	}

	userID := jwtUserData.UserID

	search := c.Query("search")

	page, err := conv.StringToInt64(c.Query("page", "1"))
	if err != nil || page <= 0 {
		page = 1
	}

	limit, err := conv.StringToInt64(c.Query("limit", "10"))
	if err != nil || limit <= 0 {
		limit = 10
	}

	status := ""
	if statusStr := c.Query("status"); statusStr != "" {
		status = statusStr
	}

	reqEntity := entity.QueryStringEntity{
		Search:  search,
		Status:  status,
		Page:    page,
		Limit:   limit,
		BuyerID: userID,
	}

	results, totalData, totalPage, err := o.orderService.GetAllCustomer(ctx, reqEntity)
	if err != nil {
		log.Error().
			Err(err).
			Str("search", search).
			Int64("page", page).
			Int64("limit", limit).
			Str("source", "internal.adapter.orderHandler.GetAllCustomer").
			Msg("failed get order customer list")

		if err.Error() == "404" {
			return fiber.NewError(fiber.StatusNotFound, "data not found")
		}

		return err
	}

	respOrders := make([]response.OrderCustomerList, 0, len(results))

	for _, result := range results {
		respOrders = append(respOrders, response.OrderCustomerList{
			ID:            result.ID,
			OrderCode:     result.OrderCode,
			Status:        result.Status,
			ProductName:   result.OrderItems[0].ProductName,
			TotalAmount:   result.TotalAmount,
			ProductImage:  result.OrderItems[0].ProductImage,
			Weight:        result.OrderItems[0].ProductWeight,
			Unit:          result.OrderItems[0].ProductUnit,
			Quantity:      result.OrderItems[0].Quantity,
			OrderDateTime: fmt.Sprintf("%s %s", result.OrderDate, result.OrderTime),
			PaymentMethod: result.PaymentMethod,
		})
	}

	return c.Status(fiber.StatusOK).JSON(response.DefaultResponseWithPaginations{
		Message: "data retrieved successfully",
		Data:    respOrders,
		Pagination: &response.Pagination{
			Page:       page,
			TotalCount: totalData,
			PerPage:    limit,
			TotalPage:  totalPage,
		},
	})
}

func (o *orderHandler) UpdateStatus(c fiber.Ctx) error {
	var (
		ctx = c.Context()
		req = request.OrderUpdateStatusRequest{}
	)

	if err := c.Bind().Body(&req); err != nil {
		log.Error().
			Err(err).
			Str("source", "internal.adapter.orderHandler.UpdateStatus").
			Msg("failed bind/validate request")

		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	orderIDStr := c.Params("orderID")
	if orderIDStr == "" {
		log.Info().
			Str("source", "internal.adapter.orderHandler.UpdateStatus").
			Msg("missing or invalid order ID")

		return fiber.NewError(fiber.StatusBadRequest, "missing or invalid order ID")
	}

	orderID, err := conv.StringToInt64(orderIDStr)
	if err != nil {
		log.Info().
			Err(err).
			Str("order id", orderIDStr).
			Str("source", "internal.adapter.orderHandler.UpdateStatus").
			Msg("invalid order ID")

		return fiber.NewError(fiber.StatusBadRequest, "invalid order ID")
	}

	reqEntity := entity.OrderEntity{
		Remarks: req.Remarks,
		Status:  req.Status,
		ID:      orderID,
	}

	err = o.orderService.UpdateStatus(ctx, reqEntity)
	if err != nil {
		log.Error().
			Err(err).
			Str("source", "internal.adapter.orderHandler.UpdateStatus").
			Msg("failed update status")

		if err.Error() == "404" {
			return fiber.NewError(fiber.StatusNotFound, "data not found")
		}

		if err.Error() == "400" {
			return fiber.NewError(fiber.StatusBadRequest, "invalid status transition")
		}

		return err
	}

	return c.Status(fiber.StatusOK).JSON(response.DefaultResponse{
		Message: "status updated successfully",
		Data:    nil,
	})
}

func (o *orderHandler) CreateOrder(c fiber.Ctx) error {
	var (
		ctx = c.Context()
		req = request.CreateOrderRequest{}
	)

	if err := c.Bind().Body(&req); err != nil {
		log.Error().
			Err(err).
			Str("source", "internal.adapter.orderHandler.CreateOrder").
			Msg("failed bind/validate request")

		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	reqEntity := entity.OrderEntity{
		BuyerId:       req.BuyerID,
		OrderDate:     req.OrderDate,
		TotalAmount:   req.TotalAmount,
		ShippingType:  req.ShippingType,
		Remarks:       req.Remarks,
		OrderTime:     req.OrderTime,
		PaymentMethod: req.PaymentMethod,
	}

	orderDetails := []entity.OrderItemEntity{}
	for _, val := range req.OrderDetails {
		orderDetails = append(orderDetails, entity.OrderItemEntity{
			ProductID: val.ProductID,
			Quantity:  val.Quantity,
		})
	}

	reqEntity.OrderItems = orderDetails

	orderID, err := o.orderService.CreateOrder(ctx, reqEntity)
	if err != nil {
		log.Error().
			Err(err).
			Str("source", "internal.adapter.orderHandler.CreateOrder").
			Msg("failed create order")

		return err
	}

	return c.Status(fiber.StatusCreated).JSON(response.DefaultResponse{
		Message: "success",
		Data: map[string]any{
			"order_id": orderID,
		},
	})
}

func (o *orderHandler) GetByIDAdmin(c fiber.Ctx) error {
	var (
		ctx       = c.Context()
		respOrder = response.OrderAdminDetail{}
	)

	orderIDStr := c.Params("orderID")
	if orderIDStr == "" {
		log.Info().
			Str("source", "internal.adapter.orderHandler.UpdateStatus").
			Msg("missing or invalid order ID")

		return fiber.NewError(fiber.StatusBadRequest, "missing or invalid order ID")
	}

	orderID, err := conv.StringToInt64(orderIDStr)
	if err != nil {
		log.Info().
			Err(err).
			Str("order id", orderIDStr).
			Str("source", "internal.adapter.orderHandler.UpdateStatus").
			Msg("invalid order ID")

		return fiber.NewError(fiber.StatusBadRequest, "invalid order ID")
	}

	order, err := o.orderService.GetByID(ctx, orderID)
	if err != nil {
		log.Error().
			Err(err).
			Str("source", "internal.adapter.orderHandler.GetByIDAdmin")

		if err.Error() == "404" {
			return fiber.NewError(fiber.StatusNotFound, "data not found")
		}
		return err
	}

	respOrder.ID = order.ID
	respOrder.OrderCode = order.OrderCode
	respOrder.Status = order.Status
	respOrder.TotalAmount = order.TotalAmount
	respOrder.OrderDatetime = order.OrderDate
	respOrder.ShippingFee = order.ShippingFee
	respOrder.Remarks = order.Remarks
	respOrder.PaymentMethod = order.PaymentMethod
	respOrder.Customer = response.CustomerOrder{
		CustomerName:    order.BuyerName,
		CustomerPhone:   order.BuyerPhone,
		CustomerAddress: order.BuyerAddress,
		CustomerEmail:   order.BuyerEmail,
		CustomerID:      order.BuyerId,
	}

	for _, item := range order.OrderItems {
		respOrder.OrderDetail = append(respOrder.OrderDetail, response.OrderDetail{
			ProductName:  item.ProductName,
			ProductImage: item.ProductImage,
			ProductPrice: item.Price,
			Quantity:     item.Quantity,
		})
	}

	return c.Status(fiber.StatusOK).JSON(response.DefaultResponse{
		Message: "success",
		Data:    respOrder,
	})
}

func (o *orderHandler) GetAllAdmin(c fiber.Ctx) error {
	var (
		ctx        = c.Context()
		respOrders = []response.OrderAdminList{}
	)

	search := c.Query("search")

	page, err := conv.StringToInt64(c.Query("page", "1"))
	if err != nil || page <= 0 {
		page = 1
	}

	limit, err := conv.StringToInt64(c.Query("limit", "10"))
	if err != nil || limit <= 0 {
		limit = 10
	}

	status := ""
	if statusStr := c.Query("status"); statusStr != "" {
		status = statusStr
	}

	reqEntity := entity.QueryStringEntity{
		Search: search,
		Status: status,
		Page:   page,
		Limit:  limit,
	}

	results, totalData, totalPage, err := o.orderService.GetAll(ctx, reqEntity)
	if err != nil {
		log.Error().
			Err(err).
			Str("search", search).
			Int64("page", page).
			Int64("limit", limit).
			Str("source", "internal.adapter.orderHandler.GetAllAdmin").
			Msg("failed get order list")

		if err.Error() == "404" {
			return fiber.NewError(fiber.StatusNotFound, "data not found")
		}
		return err
	}

	for _, result := range results {
		var productImage string
		for _, val := range result.OrderItems {
			productImage = val.ProductImage
		}

		respOrders = append(respOrders, response.OrderAdminList{
			ID:            result.ID,
			OrderCode:     result.OrderCode,
			OrderDateTime: fmt.Sprintf("%s %s", result.OrderDate, result.OrderTime),
			Status:        result.Status,
			TotalAmount:   result.TotalAmount,
			ProductImage:  productImage,
			CustomerName:  result.BuyerName,
			PaymentMethod: result.PaymentMethod,
		})
	}

	return c.Status(fiber.StatusOK).JSON(response.DefaultResponseWithPaginations{
		Message: "data retrieved successfully",
		Data:    respOrders,
		Pagination: &response.Pagination{
			Page:       page,
			TotalCount: totalData,
			PerPage:    limit,
			TotalPage:  totalPage,
		},
	})
}

func (o *orderHandler) GetOrderByOrderCodeInternal(c fiber.Ctx) error {
	ctx := c.Context()

	orderCode := c.Params("orderCode")
	if orderCode == "" {
		log.Info().
			Str("source", "internal.adapter.orderHandler.GetOrderByOrderCodeInternal").
			Msg("missing or invalid order code")

		return fiber.NewError(fiber.StatusBadRequest, "missing or invalid order code")
	}

	order, err := o.orderService.GetOrderByOrderCode(ctx, orderCode)
	if err != nil {
		log.Error().
			Err(err).
			Str("order code", orderCode).
			Str("source", "internal.adapter.orderHandler.GetOrderByOrderCodeInternal").
			Msg("failed get order")

		if err.Error() == "404" {
			return fiber.NewError(fiber.StatusNotFound, "order not found")
		}

		return err
	}

	respOrder := response.OrderAdminDetail{
		ID:            order.ID,
		OrderCode:     order.OrderCode,
		ProductImage:  "",
		OrderDatetime: order.OrderDate,
		Status:        order.Status,
		PaymentMethod: order.PaymentMethod,
		ShippingFee:   order.ShippingFee,
		ShippingType:  order.ShippingType,
		Remarks:       order.Remarks,
		TotalAmount:   order.TotalAmount,
		Customer: response.CustomerOrder{
			CustomerName:    order.BuyerName,
			CustomerPhone:   order.BuyerPhone,
			CustomerAddress: order.BuyerAddress,
			CustomerEmail:   order.BuyerEmail,
			CustomerID:      order.BuyerId,
		},
		OrderDetail: make([]response.OrderDetail, 0, len(order.OrderItems)),
	}

	for _, item := range order.OrderItems {
		respOrder.OrderDetail = append(respOrder.OrderDetail, response.OrderDetail{
			ProductName:  item.ProductName,
			ProductImage: item.ProductImage,
			ProductPrice: item.Price,
			Quantity:     item.Quantity,
		})

		if respOrder.ProductImage == "" {
			respOrder.ProductImage = item.ProductImage
		}
	}

	return c.Status(fiber.StatusOK).JSON(response.DefaultResponse{
		Message: "success get order by order code",
		Data:    respOrder,
	})
}

func (o *orderHandler) GetPublicOrderByOrderCodeInternal(c fiber.Ctx) error {
	ctx := c.Context()

	orderCode := c.Params("orderCode")
	if orderCode == "" {
		log.Info().
			Str("source", "internal.adapter.orderHandler.GetPublicOrderByOrderCodeInternal").
			Msg("missing or invalid order code")
		return fiber.NewError(fiber.StatusBadRequest, "missing or invalid order code")
	}

	order, err := o.orderService.GetPublicOrderIDByOrderCode(ctx, orderCode)
	if err != nil {
		log.Error().
			Err(err).
			Str("orderCode", orderCode).
			Str("source", "internal.adapter.orderHandler.GetPublicOrderByOrderCodeInternal").
			Msg("failed get order ID")
		if err.Error() == "404" {
			return fiber.NewError(fiber.StatusNotFound, "order ID not found")
		}
		return err
	}

	return c.Status(fiber.StatusOK).JSON(response.DefaultResponse{
		Message: "success get order id",
		Data: map[string]int64{
			"orderID": order,
		},
	})
}
