package dto

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

type NullInt32 struct {
	sql.NullInt32
}

// UnmarshalJSON handles both null and int JSON values
func (n *NullInt32) UnmarshalJSON(data []byte) error {
	// Check for null value
	if string(data) == "null" {
		n.Valid = false
		n.Int32 = 0
		return nil
	}

	// Try to parse the integer value
	var num int32
	if err := json.Unmarshal(data, &num); err != nil {
		return fmt.Errorf("NullInt32: cannot unmarshal %s into int32", data)
	}

	n.Int32 = num
	n.Valid = true
	return nil
}

// MarshalJSON ensures that if Valid is false, it serializes as null
func (n NullInt32) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(n.Int32)
}

type CreateInvoiceDto struct {
	CreatedAt string `json:"created_at"`
	Types string `json:"types"`
	LocationId string `json:"location_id"`
	Qty int32 `json:"qty"`
	StockDocumentType string `json:"stock_document_type"`
	Price float64 `json:"price"`
	SalesReturnId NullInt32 `json:"sales_return_id"`
	PurchaseReturnId NullInt32 `json:"purchase_return_id"`
}
