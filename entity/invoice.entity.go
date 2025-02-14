package entity

import (
	"database/sql"
)

type Invoice struct {
	Id int32 `json:"id"`
	CreatedAt string `json:"created_at"`
	Types string `json:"types"`
	LocationId string `json:"location_id"`
	Qty sql.NullInt32 `json:"qty"`
	StockDocumentType string `json:"stock_document_type"`
	Price sql.NullFloat64 `json:"price"`
	Cogs sql.NullFloat64 `json:"cogs"`
	RemainingQty sql.NullInt32 `json:"remaining_qty"`
	FifoInputStockMovementId sql.NullInt32 `json:"fifo_input_stock_movement_id"`
	FifoInputPreAdjustmentRemainingQty sql.NullInt32 `json:"fifo_input_pre_adjustment_remaining_qty"`
	SalesReturnId sql.NullInt32 `json:"sales_return_id"`
	PurchaseReturnId sql.NullInt32 `json:"purchase_return_id"`
	AccumulatedQty sql.NullInt32 `json:"accumulated_qty"`
	AccumulatedInventoryValue sql.NullFloat64 `json:"accumulated_inventory_value"`
	UsedPhantomQty sql.NullInt32 `json:"used_phantom_qty"`
}
