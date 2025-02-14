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
		"INSERT INTO invoices (created_at, types, location_id, qty, stock_document_type, accumulated_qty, accumulated_inventory_value, used_phantom_qty) VALUES ($1, 'input', $2, $3, 'Phantom Purchase', $4, $5, $3)",
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
			&invoice.UsedPhantomQty,
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

func GetAllInvoicesByCreatedAt(createdAt string, operator string) ([]entity.Invoice, error) {
	query := fmt.Sprintf("SELECT * FROM invoices WHERE created_at %s '%s' ORDER BY created_at ASC, id ASC", operator, createdAt)
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
			&invoice.UsedPhantomQty,
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

func GetOutstandingPhantomPurchase(createdAt string) (entity.Invoice) {
	var invoice entity.Invoice
	db.Db.QueryRow("SELECT * FROM invoices WHERE used_phantom_qty > 0 AND stock_document_type LIKE 'Phantom%' AND created_at <= $1 ORDER BY created_at ASC LIMIT 1", createdAt).Scan(
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
			&invoice.UsedPhantomQty,
		)

	return invoice
}

func GetLatestInvoice(createdAt string) (entity.Invoice) {
	var invoice entity.Invoice
	db.Db.QueryRow("SELECT * FROM invoices WHERE created_at <= $1 ORDER BY created_at DESC, id DESC LIMIT 1", createdAt).Scan(
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
			&invoice.UsedPhantomQty,
		)

	return invoice
}

func GetLatestUniqueInvoice(createdAt string, id int32) (entity.Invoice) {
	var invoice entity.Invoice
	db.Db.QueryRow("SELECT * FROM invoices WHERE created_at <= $1 AND id != $2 ORDER BY created_at DESC, id DESC LIMIT 1", createdAt, id).Scan(
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
			&invoice.UsedPhantomQty,
		)

	return invoice
}

func GetFirstNonEmptyBatch() (entity.Invoice, error) {
	var invoice entity.Invoice
	queryError := db.Db.QueryRow("SELECT * FROM invoices WHERE remaining_qty > 0 AND stock_document_type NOT LIKE '%Phantom%' ORDER BY created_at ASC, id ASC LIMIT 1").Scan(
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
			&invoice.UsedPhantomQty,
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
	queryError := db.Db.QueryRow("SELECT * FROM invoices WHERE id = $1", id).Scan(
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
			&invoice.UsedPhantomQty,
		)

	if queryError != nil {
		return entity.Invoice{}, queryError
	} else {
		return invoice, nil
	}
}

func UpdateInvoice(cogs *float64, remainingQty int32, fifoInputMovementId *int32, fifoInputPreAdjustmentRemainingQty *int32, accumulatedQty int32, accumulatedInventoryValue float64, id int32) (string, error) {
	_, err := db.Db.Exec(
		"UPDATE invoices SET cogs = $1, remaining_qty = $2, fifo_input_stock_movement_id = $3, fifo_input_pre_adjustment_remaining_qty = $4, accumulated_qty = $5, accumulated_inventory_value = $6 WHERE id = $7",
		cogs,
		remainingQty,
		fifoInputMovementId,
		fifoInputPreAdjustmentRemainingQty,
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
		"DELETE FROM invoices WHERE created_at > $1 AND stock_document_type LIKE '%Phantom%'",
		createdAt,
	)
	if err != nil {
		return "delete phantom invoice failed", err
	}
	return "delete phantom invoice success", nil
}

func GetLastOutputInvoice(createdAt string) (entity.Invoice, error) {
	var invoice entity.Invoice
	queryError := db.Db.QueryRow("SELECT * FROM invoices WHERE created_at > $1 AND types = 'output' ORDER BY created_at DESC, id DESC LIMIT 1 OFFSET 0", createdAt).Scan(
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
			&invoice.UsedPhantomQty,
		)

	if queryError != nil {
		return entity.Invoice{}, queryError
	} else {
		return invoice, nil
	}
}

func GetNewerNonPhantomOutputInvoice(createdAt string) (entity.Invoice, error) {
	var invoice entity.Invoice
	queryError := db.Db.QueryRow("SELECT * FROM invoices WHERE created_at > $1 AND types = 'output' AND stock_document_type NOT LIKE '%Phantom%' ORDER BY created_at ASC, id ASC LIMIT 1 OFFSET 0", createdAt).Scan(
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
			&invoice.UsedPhantomQty,
		)

	if queryError != nil {
		return entity.Invoice{}, queryError
	} else {
		return invoice, nil
	}
}

func ResetRemainingQty(createdAt string, id int32) (string, error) {
	_, err := db.Db.Exec(
		"UPDATE invoices SET remaining_qty = qty WHERE created_at > $1 AND types = 'input' AND stock_document_type NOT LIKE '%Phantom%' AND id != $2",
		createdAt,
		id,
	)
	if err != nil {
		return "update invoice failed", err
	}

	return "reset remaining qty success", nil
}

func UpdateInvoiceUsedPhantomQty(qty int32, id int32) (string, error) {
	_, err := db.Db.Exec(
		"UPDATE invoices SET used_phantom_qty = $1 WHERE id = $2",
		qty,
		id,
	)
	if err != nil {
		return "update invoice used phantom qty failed", err
	}

	return "update invoice used phantom qty success", nil
}