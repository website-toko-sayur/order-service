package model

import "strconv"

type ProductUpdateStockEvent struct {
	ProductID int64 `json:"productID"`
	Quantity  int64 `json:"quantity"`
}

func (u *ProductUpdateStockEvent) GetId() string {
	return strconv.Itoa(int(u.ProductID))
}
