package model

import (
	"strconv"
	"time"
)

type OrderPublishEvent struct {
	ID            int64            `json:"id"`
	OrderCode     string           `json:"order_code"`
	BuyerId       int64            `json:"buyer_id"`
	OrderDate     string           `json:"order_date"`
	Status        string           `json:"status"`
	TotalAmount   int64            `json:"total_amount"`
	PaymentMethod string           `json:"payment_method"`
	ShippingType  string           `json:"shipping_type"`
	ShippingFee   int64            `json:"shipping_fee"`
	OrderTime     string           `json:"order_time"`
	Remarks       string           `json:"remarks"`
	CreatedAt     time.Time        `json:"created_at"`
	OrderItems    []OrderItemEvent `json:"order_items"`
	BuyerName     string           `json:"buyer_name"`
	BuyerEmail    string           `json:"buyer_email"`
	BuyerPhone    string           `json:"buyer_phone"`
	BuyerAddress  string           `json:"buyer_address"`
	BuyerLat      string           `json:"buyer_lat"`
	BuyerLng      string           `json:"buyer_lng"`
}

type OrderItemEvent struct {
	ID            int64  `json:"id"`
	OrderID       int64  `json:"order_id"`
	ProductID     int64  `json:"product_id"`
	Quantity      int64  `json:"quantity"`
	OrderCode     string `json:"order_code"`
	ProductName   string `json:"product_name"`
	ProductImage  string `json:"product_image"`
	Price         int64  `json:"price"`
	ProductUnit   string `json:"product_unit"`
	ProductWeight int64  `json:"product_weight"`
}

func (u *OrderPublishEvent) GetId() string {
	return strconv.Itoa(int(u.ID))
}
