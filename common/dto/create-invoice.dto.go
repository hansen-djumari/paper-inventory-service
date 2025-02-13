package dto

import (
	"database/sql"
)

type CreateInvoiceDto struct {
	CreatedAt string `json:"created_at"`
	Types string `json:"types"`
	LocationId string `json:"location_id"`
	Qty int32 `json:"qty"`
	StockDocumentType string `json:"stock_document_type"`
	Price float64 `json:"price"`
	SalesReturnId sql.NullInt32 `json:"sales_return_id"`
	PurchaseReturnId sql.NullInt32 `json:"purchase_return_id"`
}
