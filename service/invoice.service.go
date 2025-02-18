package service

import (
	"database/sql"
	"fmt"
	"math"
	"paper/inventory-api/common"
	"paper/inventory-api/common/dto"
	"paper/inventory-api/db"
	"paper/inventory-api/entity"
	"paper/inventory-api/repository"
	"strings"
)

func GetPagebaleInvoicesByCreateDate(createdAt string, comparison string, size int, page int) ([]entity.Invoice, error) {
	query := fmt.Sprintf("SELECT * FROM invoices WHERE created_at %s '%s' ORDER BY created_at ASC LIMIT %d OFFSET %d", comparison, createdAt, size, page)
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
		return invoices, err
	}

	return invoices, nil
}

func CreateRecord(createInvoicePayload dto.CreateInvoiceDto) (string, error) {
	var message string
	var err error

	tx, err := db.Db.Begin()
	if err != nil {
		return "create record failed", err
	}
	recordAfter, err := repository.GetPageableInvoicesByCreatedAt(tx, createInvoicePayload.CreatedAt, ">", 1, 0)
	if err != nil {
		return "create record failed", err
	}
	recordBefore := repository.GetLatestInvoice(tx, createInvoicePayload.CreatedAt)

	outStandingPhantom := repository.GetOutstandingPhantomPurchase(tx, createInvoicePayload.CreatedAt)

	stockDocumentType := createInvoicePayload.StockDocumentType
	isBackdate := len(recordAfter) > 0
	isNegative := recordBefore.AccumulatedQty.Int32 < createInvoicePayload.Qty
	isPhantomExist := outStandingPhantom != entity.Invoice{}

	var cogs *float64
	var fifoInputStockMovementId *int32
	var fifoInputPreAdjustmentRemainingQty *int32

	if (isBackdate) {
		/*
			delete all newer phantom records
			get newer non phantom output record -> update remaining_qty of the record
			reset remaining_qty of the record after that
			handle reset remaining qty
				get last output record after the backdated record
					get last updated batch using output.fifo_stock_movement_id reset it to output.fifo_input_pre_adjustment_remaining_qty
					reset the rest of input record remaining_qty to its qty
		*/

		// handle backdate
		_, err = repository.DeleteNewerPhantomInvoice(tx, createInvoicePayload.CreatedAt)
		if err != nil {
			return "delete newer phantom record failed", err
		}

		newerNonPhantomOutputRecord := repository.GetNewerNonPhantomOutputInvoice(tx, createInvoicePayload.CreatedAt)
		if (newerNonPhantomOutputRecord != entity.Invoice{}) {
			_, err = repository.UpdateInvoiceRemainingQty(tx, newerNonPhantomOutputRecord.FifoInputPreAdjustmentRemainingQty.Int32, newerNonPhantomOutputRecord.FifoInputStockMovementId.Int32)
			if err != nil {
				return "update remaining qty last input record failed", err
			}

			_, err = repository.ResetRemainingQty(tx, newerNonPhantomOutputRecord.CreatedAt, newerNonPhantomOutputRecord.Id)
			if err != nil {
				return "reset remaining qty failed", err
			}	
		}

		recordAfter, err = repository.GetAllInvoicesByCreatedAt(tx, createInvoicePayload.CreatedAt, ">")
		if err != nil {
			return "create record failed", err
		}
	}

	switch strings.ToLower(stockDocumentType) {
	case common.Purchase:
		// handle create new record
		message, err = PurchaseHandler(createInvoicePayload, recordBefore, outStandingPhantom, cogs, fifoInputStockMovementId, fifoInputPreAdjustmentRemainingQty, isPhantomExist, nil, tx)
		if err != nil {
			return message, err
		}

	case common.Sales:
		SalesAndPurchaseReturnHandler(createInvoicePayload, recordBefore, isNegative, nil, tx)
	
	case common.SalesReturn:
		// handle create new record
		message, err := SalesReturnHandler(createInvoicePayload, recordBefore, outStandingPhantom, isPhantomExist, nil, tx)
		if err != nil {
			return message, err
		}

	case common.PurchaseReturn:
		SalesAndPurchaseReturnHandler(createInvoicePayload, recordBefore, isNegative, nil, tx)
	}

	for _, record := range recordAfter {
		payload := dto.CreateInvoiceDto{
			CreatedAt: record.CreatedAt,
			Types: record.Types,
			LocationId: record.LocationId,
			Qty: record.Qty.Int32,
			StockDocumentType: record.StockDocumentType,
			Price: record.Price.Float64,
			SalesReturnId: dto.NullInt32{
				NullInt32: sql.NullInt32{
					Valid: true,
					Int32: record.SalesReturnId.Int32,
				},
			},
			PurchaseReturnId: dto.NullInt32{
				NullInt32: sql.NullInt32{
					Valid: true,
					Int32: record.PurchaseReturnId.Int32,
				},
			},
		}

		recordBefore = repository.GetLatestUniqueInvoice(tx, record.CreatedAt, record.Id)

		outStandingPhantom := repository.GetOutstandingPhantomPurchase(tx, record.CreatedAt)

		stockDocumentType = record.StockDocumentType
		isNegative = recordBefore.AccumulatedQty.Int32 < record.Qty.Int32
		isPhantomExist = outStandingPhantom != entity.Invoice{}

		switch strings.ToLower(stockDocumentType) {
		case common.Purchase:
			// handle create new record
			message, err = PurchaseHandler(payload, recordBefore, outStandingPhantom, cogs, fifoInputStockMovementId, fifoInputPreAdjustmentRemainingQty, isPhantomExist, &record.Id, tx)
			if err != nil {
				return message, err
			}

		case common.Sales:
			SalesAndPurchaseReturnHandler(payload, recordBefore, isNegative, &record.Id, tx)
		
		case common.SalesReturn:
			// handle create new record
			message, err := SalesReturnHandler(payload, recordBefore, outStandingPhantom, isPhantomExist, &record.Id, tx)
			if err != nil {
				return message, err
			}

		case common.PurchaseReturn:
			SalesAndPurchaseReturnHandler(payload, recordBefore, isNegative, &record.Id, tx)
		}
	}

	tx.Commit()

	return message, err
}

func PurchaseHandler(createInvoicePayload dto.CreateInvoiceDto, recordBefore entity.Invoice, outStandingPhantom entity.Invoice, cogs *float64, fifoInputStockMovementId *int32, fifoInputPreAdjustmentRemainingQty *int32, isPhantomExist bool, id *int32,  tx *sql.Tx) (string, error) {
	// handle create new record
	price := createInvoicePayload.Price
	remainingQty := createInvoicePayload.Qty
	accumulatedQty, accumulatedInventoryValue := CalculateAccumulativeValue(createInvoicePayload.Types, recordBefore.AccumulatedQty.Int32, recordBefore.AccumulatedInventoryValue.Float64, createInvoicePayload.Qty, price, cogs)

	var invoice entity.Invoice
	if id != nil {
		invoice = repository.UpdateInvoice(tx, cogs, remainingQty, nil, nil, accumulatedQty, accumulatedInventoryValue, nil, *id, 0)
	} else {
		invoice = repository.InsertInvoice(tx, createInvoicePayload, cogs, remainingQty, fifoInputStockMovementId, fifoInputPreAdjustmentRemainingQty, accumulatedQty, accumulatedInventoryValue, nil)
	}

	// handle create clear phantom
	if (isPhantomExist) {
		_, err := ClearPhantomHandler(accumulatedQty, accumulatedInventoryValue, price, outStandingPhantom, createInvoicePayload, invoice.RemainingQty.Int32, invoice.Id, tx)
		if err != nil {
			return "create record failed", err
		}
	}
	return "create record success", nil
}

func SalesReturnHandler(createInvoicePayload dto.CreateInvoiceDto, recordBefore entity.Invoice, outStandingPhantom entity.Invoice, isPhantomExist bool, id *int32, tx *sql.Tx) (string, error) {
	batchBeingReturnedTo, err := repository.GetInvoiceById(tx, createInvoicePayload.SalesReturnId.Int32)
	if err != nil {
		return "create record failed", err
	}

	createInvoicePayload.Price = math.Round(batchBeingReturnedTo.Cogs.Float64 / float64(batchBeingReturnedTo.Qty.Int32))
	remainingQty := createInvoicePayload.Qty
	accumulatedQty, accumulatedInventoryValue := CalculateAccumulativeValue(createInvoicePayload.Types, recordBefore.AccumulatedQty.Int32, recordBefore.AccumulatedInventoryValue.Float64, createInvoicePayload.Qty, createInvoicePayload.Price, nil)

	var invoice entity.Invoice
	if id != nil {
		invoice = repository.UpdateInvoice(tx, nil, remainingQty, nil, nil, accumulatedQty, accumulatedInventoryValue, nil, *id, 0)
	} else {
		invoice = repository.InsertInvoice(tx, createInvoicePayload, nil, remainingQty, nil, nil, accumulatedQty, accumulatedInventoryValue, nil)
	}

	// handle create clear phantom
	if (isPhantomExist) {
		_, err := ClearPhantomHandler(accumulatedQty, accumulatedInventoryValue, createInvoicePayload.Price, outStandingPhantom, createInvoicePayload, invoice.RemainingQty.Int32, invoice.Id, tx)
		if err != nil {
			return "create record failed", err
		}
	}
	return "create record success", err
}

func SalesAndPurchaseReturnHandler(createInvoicePayload dto.CreateInvoiceDto, recordBefore entity.Invoice, isNegative bool, id *int32, tx *sql.Tx) (entity.Invoice) {
	firstInOutStandingBatch, _ := repository.GetFirstNonEmptyBatch(tx)

	var totalCogsValue float64

	var phantomQty int32
	var accumulatedQty int32
	var accumulatedInventoryValue float64
	var phantomId int32
	var invoiceId int32
	if (isNegative) {
		// create phantom invoice
		phantomQty, invoiceId, _ = NegativeStockHandler(recordBefore, createInvoicePayload, tx)
		phantomId = invoiceId + 1
		totalCogsValue = CalculateTotalCogsForOutputRecord(createInvoicePayload, firstInOutStandingBatch, phantomQty, tx)
		accumulatedQty = 0
		accumulatedInventoryValue = 0
	} else {
		totalCogsValue = CalculateTotalCogsForOutputRecord(createInvoicePayload, firstInOutStandingBatch, 0, tx)
		accumulatedQty, accumulatedInventoryValue = CalculateAccumulativeValue(createInvoicePayload.Types, recordBefore.AccumulatedQty.Int32, recordBefore.AccumulatedInventoryValue.Float64, createInvoicePayload.Qty, 0, &totalCogsValue)
	}

	// using pointer to handle nullable value
	var firstFifoInputStockMovementId int32
	var firstFifoPreAdjusmentRemainingQty int32
	firstFifoInputStockMovementId = firstInOutStandingBatch.Id
	firstFifoPreAdjusmentRemainingQty = firstInOutStandingBatch.RemainingQty.Int32
	if createInvoicePayload.StockDocumentType == common.PurchaseReturn {
		createInvoicePayload.Price = totalCogsValue / float64(createInvoicePayload.Qty)
	}

	var invoice entity.Invoice
	if id != nil {
		invoice = repository.UpdateInvoice(tx, &totalCogsValue, 0, &firstFifoInputStockMovementId, &firstFifoPreAdjusmentRemainingQty, accumulatedQty, accumulatedInventoryValue, &phantomQty, *id, phantomId)
	} else {
		invoice = repository.InsertInvoice(tx, createInvoicePayload, &totalCogsValue, 0, &firstFifoInputStockMovementId, &firstFifoPreAdjusmentRemainingQty, accumulatedQty, accumulatedInventoryValue, &phantomQty)
	}
	return invoice
}

func NegativeStockHandler(recordBefore entity.Invoice, createInvoicePayload dto.CreateInvoiceDto, tx *sql.Tx) (int32, int32, error) {
	// create phantom invoice
	phantomQty := int32(math.Abs(float64(recordBefore.AccumulatedQty.Int32 - createInvoicePayload.Qty)))
	invoice, err := repository.InsertPhantomPurchase(tx, createInvoicePayload.CreatedAt, createInvoicePayload.LocationId, phantomQty, createInvoicePayload.Qty, recordBefore.AccumulatedInventoryValue.Float64)
	if err != nil {
		return 0, 0, err
	}
	return phantomQty, invoice.Id, err
}

func ClearPhantomHandler(accumulatedQty int32, accumulatedInventoryValue float64, price float64, outStandingPhantom entity.Invoice, createInvoicePayload dto.CreateInvoiceDto, remainingQty int32, id int32, tx *sql.Tx) (string, error) {
	var phantomQty int32
	if accumulatedQty >= outStandingPhantom.UsedPhantomQty.Int32 {
		phantomQty = outStandingPhantom.UsedPhantomQty.Int32
	} else {
		phantomQty = accumulatedQty
	}
	phantomCogs := price * float64(phantomQty)
	phantomAccumulatedQty := accumulatedQty - phantomQty
	phantomAccumulatedInventoryValue := accumulatedInventoryValue - phantomCogs
	message, err := repository.InsertClearPhantomPurchase(tx, createInvoicePayload.CreatedAt, phantomQty, phantomCogs, phantomAccumulatedQty, phantomAccumulatedInventoryValue)
	if err != nil {
		return message, err
	}

	qty := remainingQty - phantomQty
	message, err = repository.UpdateInvoiceRemainingQty(tx, qty, id)
	if err != nil {
		return message, err
	}

	usedPhantomQty := outStandingPhantom.UsedPhantomQty.Int32 - phantomQty
	message, err = repository.UpdateInvoiceUsedPhantomQty(tx, usedPhantomQty, outStandingPhantom.Id)

	return message, err
}

func CalculateAccumulativeValue(types string, previousAccumulatedQty int32, previousAccumulatedInventoryValue float64, qty int32, price float64, cogs *float64) (accumulatedQty int32, accumulatedInventoryValue float64) {
	 if types == common.TypesOutput {
		accumulatedQty = previousAccumulatedQty - qty
		accumulatedInventoryValue = previousAccumulatedInventoryValue - *cogs
	 } else {
		accumulatedQty = previousAccumulatedQty + qty
		accumulatedInventoryValue = previousAccumulatedInventoryValue + (float64(qty) * price)
	 }

	 return accumulatedQty, accumulatedInventoryValue
}

func CalculateTotalCogsForOutputRecord(createInvoicePayload dto.CreateInvoiceDto, firstInOutStandingBatch entity.Invoice, phantomQty int32, tx *sql.Tx) (float64) {
	var totalCogsValue float64
	qtyLeft := createInvoicePayload.Qty - phantomQty
	for qtyLeft > 0 && (firstInOutStandingBatch != entity.Invoice{}) {
		if firstInOutStandingBatch.RemainingQty.Int32 > qtyLeft {
			totalCogsValue += firstInOutStandingBatch.Price.Float64 * (float64(qtyLeft))
			firstInOutStandingBatchRemainingQty := firstInOutStandingBatch.RemainingQty.Int32 - createInvoicePayload.Qty
			repository.UpdateInvoiceRemainingQty(tx, firstInOutStandingBatchRemainingQty, firstInOutStandingBatch.Id)
			qtyLeft = 0
		} else {
			totalCogsValue += firstInOutStandingBatch.Price.Float64 * (float64(firstInOutStandingBatch.RemainingQty.Int32))
			qtyLeft = qtyLeft - firstInOutStandingBatch.RemainingQty.Int32
			repository.UpdateInvoiceRemainingQty(tx, 0, firstInOutStandingBatch.Id)
		}

		firstInOutStandingBatch, _ = repository.GetFirstNonEmptyBatch(tx)
	}
	return totalCogsValue
}