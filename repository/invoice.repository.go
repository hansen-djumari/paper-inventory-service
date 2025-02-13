package repository

import (
	"fmt"
	"paper/inventory-api/common/dto"
	"paper/inventory-api/db"
	"paper/inventory-api/entity"
)

func InsertInvoice(createInvoicePayload dto.CreateInvoiceDto, cogs *float64, remainingQty int32, fifoInputStockMovementId *int32, fifoInputPreAdjustmentRemainingQty *int32, accumulatedQty int32, accumulatedInventoryValue float64) (string, error) {
	_, err := db.Db.Exec(
		"INSERT INTO invoices (created_at, types, location_id, qty, stock_document_type, price, cogs, remaining_qty, fifo_input_stock_movement_id, fifo_input_pre_adjustment_remaining_qty, sales_return_id, purchase_return_id, accumulated_qty, accumulated_inventory_value) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)",
		createInvoicePayload.CreatedAt,
		createInvoicePayload.Types,
		createInvoicePayload.LocationId,
		createInvoicePayload.Qty,
		createInvoicePayload.StockDocumentType,
		createInvoicePayload.Price,
		cogs,
		remainingQty,
		fifoInputStockMovementId,
		fifoInputPreAdjustmentRemainingQty,
		createInvoicePayload.SalesReturnId,
		createInvoicePayload.PurchaseReturnId,
		accumulatedQty,
		accumulatedInventoryValue,
	)

	if err != nil {
		return "create invoice failed", err
	}

	return "create invoice success", nil
}

func InsertPhantomPurchase(createdAt string, locationId string, qty int32, accumulatedQty int32, accumulatedInventoryValue float64) (string, error) {
	_, err := db.Db.Exec(
		"INSERT INTO invoices (created_at, types, location_id, qty, stock_document_type, remaining_qty, accumulated_qty, accumulated_inventory_value) VALUES ($1, 'input', $2, $3, 'Phantom Purchase', $3, $4, $5)",
		createdAt,
		locationId,
		qty,
		accumulatedQty,
		accumulatedInventoryValue,
	)

	if err != nil {
		return "create phantom purchase success", err
	}

	return "create phantom purchase failed", nil
}

func InsertClearPhantomPurchase(createdAt string, qty int32, cogs float64, accumulatedQty int32, accumulatedInventoryValue float64) (string, error) {
	_, err := db.Db.Exec(
		"INSERT INTO invoices (created_at, types, location_id, qty, price, cogs, stock_document_type, remaining_qty, accumulated_qty, accumulated_inventory_value) VALUES ($1, 'output', 'jakarta', $2, NULL, $3, 'Clear Phantom Purchase', NULL, $4, $5)",
		createdAt,
		qty,
		cogs,
		accumulatedQty,
		accumulatedInventoryValue,
	)

	if err != nil {
		return "create phantom purchase success", err
	}

	return "create phantom purchase failed", nil
}

func GetInvoicesByCreatedAt(createdAt string, operator string, size int, offset int) ([]entity.Invoice, error) {
	query := fmt.Sprintf("SELECT * FROM invoices WHERE created_at %s '%s' ORDER BY created_at ASC, id ASC LIMIT %d OFFSET %d", operator, createdAt, size, offset)
	rows, err := db.Db.Query(query)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var invoices []entity.Invoice
	for rows.Next() {
		var invoice entity.Invoice
		if err := rows.Scan(
			&invoice.Id,
			&invoice.CreatedAt,
			&invoice.Types,
			&invoice.LocationId,
			&invoice.Qty,
			&invoice.StockDocumentType,
			&invoice.Price,
			&invoice.Cogs,
			&invoice.RemainingQty,
			&invoice.FifoInputStockMovementId,
			&invoice.FifoInputPreAdjustmentRemainingQty,
			&invoice.SalesReturnId,
			&invoice.PurchaseReturnId,
			&invoice.AccumulatedQty,
			&invoice.AccumulatedInventoryValue,
		); err != nil {
			return nil, err
		}

		invoices = append(invoices, invoice)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return invoices, nil
}

func GetOutstandingPhantomPurchase() (entity.Invoice) {
	var invoice entity.Invoice
	db.Db.QueryRow("SELECT id, types, qty, price, cogs, remaining_qty, accumulated_qty, accumulated_inventory_value FROM invoices WHERE remaining_qty > 0 AND stock_document_type LIKE 'Phantom%' ORDER BY created_at ASC LIMIT 1").Scan(
			&invoice.Id,
			&invoice.Types,
			&invoice.Qty,
			&invoice.Price,
			&invoice.Cogs,
			&invoice.RemainingQty,
			&invoice.AccumulatedQty,
			&invoice.AccumulatedInventoryValue,
		)

	return invoice
}

func GetLatestInvoice(createdAt string) (entity.Invoice) {
	var invoice entity.Invoice
	db.Db.QueryRow("SELECT types, qty, price, cogs, remaining_qty, accumulated_qty, accumulated_inventory_value FROM invoices WHERE created_at <= $1 ORDER BY created_at DESC, id DESC LIMIT 1", createdAt).Scan(
			&invoice.Types,
			&invoice.Qty,
			&invoice.Price,
			&invoice.Cogs,
			&invoice.RemainingQty,
			&invoice.AccumulatedQty,
			&invoice.AccumulatedInventoryValue,
		)

	return invoice
}

func GetFirstNonEmptyBatch() (entity.Invoice, error) {
	var invoice entity.Invoice
	queryError := db.Db.QueryRow("SELECT id, types, qty, price, cogs, remaining_qty, accumulated_qty, accumulated_inventory_value FROM invoices WHERE remaining_qty > 0 AND stock_document_type NOT LIKE '%Phantom%' ORDER BY created_at ASC, id ASC LIMIT 1").Scan(
			&invoice.Id,
			&invoice.Types,
			&invoice.Qty,
			&invoice.Price,
			&invoice.Cogs,
			&invoice.RemainingQty,
			&invoice.AccumulatedQty,
			&invoice.AccumulatedInventoryValue,
		)

	if queryError != nil {
		return entity.Invoice{}, queryError
	} else {
		return invoice, nil
	}
}

func UpdateInvoiceRemainingQty(qty int32, id int32) (string, error) {
	_, err := db.Db.Exec(
		"UPDATE invoices SET remaining_qty = $1 WHERE id = $2",
		qty,
		id,
	)
	if err != nil {
		return "update invoice failed", err
	}

	return "update invoice success", nil
}

func GetInvoiceById(id int32) (entity.Invoice, error) {
	var invoice entity.Invoice
	queryError := db.Db.QueryRow("SELECT id, types, qty, price, cogs, remaining_qty, accumulated_qty, accumulated_inventory_value FROM invoices WHERE id = $1", id).Scan(
			&invoice.Id,
			&invoice.Types,
			&invoice.Qty,
			&invoice.Price,
			&invoice.Cogs,
			&invoice.RemainingQty,
			&invoice.AccumulatedQty,
			&invoice.AccumulatedInventoryValue,
		)

	if queryError != nil {
		return entity.Invoice{}, queryError
	} else {
		return invoice, nil
	}
}

func UpdateInvoice(cogs *float64, remainingQty int32, fifoInputMovementId *int, accumulatedQty int32, accumulatedInventoryValue float64, id int32) (string, error) {
	_, err := db.Db.Exec(
		"UPDATE invoices SET cogs = $1, remaining_qty = $2, fifo_input_stock_movement_id = $3, accumulated_qty = $4, accumulated_inventory_value = $5 WHERE id = $6",
		cogs,
		remainingQty,
		fifoInputMovementId,
		accumulatedQty,
		accumulatedInventoryValue,
		id,
	)
	if err != nil {
		return "update invoice failed", err
	}
	return "update invoice success", nil
}

func DeleteNewerPhantomInvoice(createdAt string) (string, error) {
	_, err := db.Db.Exec(
		"DELETE invoices WHERE createdAt > $1",
		createdAt,
	)
	if err != nil {
		return "update invoice failed", err
	}
	return "update invoice success", nil
}

func GetLastOutputInvoice(createdAt string) (entity.Invoice, error) {
	var invoice entity.Invoice
	queryError := db.Db.QueryRow("SELECT id, types, qty, price, cogs, remaining_qty, fifo_input_stock_movement_id, accumulated_qty, accumulated_inventory_value FROM invoices WHERE created_at > $1 AND types = 'output' ORDER BY created_at DESC, id DESC LIMIT 1 OFFSET 0").Scan(
			&invoice.Id,
			&invoice.Types,
			&invoice.Qty,
			&invoice.Price,
			&invoice.Cogs,
			&invoice.RemainingQty,
			&invoice.FifoInputStockMovementId,
			&invoice.AccumulatedQty,
			&invoice.AccumulatedInventoryValue,
		)

	if queryError != nil {
		return entity.Invoice{}, queryError
	} else {
		return invoice, nil
	}
}

func ResetRemainingQty(createdAt string) (string, error) {
	_, err := db.Db.Exec(
		"UPDATE invoices SET remaining_qty = qty WHERE created_at > $1 AND type = 'input'",
		createdAt,
	)
	if err != nil {
		return "update invoice failed", err
	}

	return "reset remaining qty success", nil
}