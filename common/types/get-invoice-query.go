package types

type GetInvoiceQuery struct {
	CreatedAt string `form:"created_at"`
	Page int `form:"page"`
	Size int `form:"size"`
}