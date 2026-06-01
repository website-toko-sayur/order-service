package entity

type UserHttpClientResponse struct {
	Message string           `json:"message"`
	Data    CustomerResponse `json:"data"`
}

type CustomerResponse struct {
	RoleID  int    `json:"role_id"`
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	Phone   string `json:"phone"`
	Lat     string `json:"lat"`
	Lng     string `json:"lng"`
	Address string `json:"address"`
	Photo   string `json:"photo"`
}
