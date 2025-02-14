package service

import (
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

func CreateRecord(createInvoicePayload dto.CreateInvoiceDto) (message string, err error) {
	recordAfter, err := repository.GetAllInvoicesByCreatedAt(createInvoicePayload.CreatedAt, ">")
	if err != nil {
		return "create record failed", err
	}
	recordBefore := repository.GetLatestInvoice(createInvoicePayload.CreatedAt)

	outStandingPhantom := repository.GetOutstandingPhantomPurchase(createInvoicePayload.CreatedAt)

	stockDocumentType := createInvoicePayload.StockDocumentType
	isBackdate := len(recordAfter) > 0
	isNegative := recordBefore.AccumulatedQty.Int32 < createInvoicePayload.Qty
	isPhantomExist := outStandingPhantom != entity.Invoice{}

	var cogs *float64
	var price float64
	var remainingQty int32
	var fifoInputStockMovementId *int32
	var fifoInputPreAdjustmentRemainingQty *int32
	var accumulatedQty int32
	var accumulatedInventoryValue float64

	switch strings.ToLower(stockDocumentType) {
	case common.Purchase:
		// handle create new record
		price = createInvoicePayload.Price
		remainingQty = createInvoicePayload.Qty
		accumulatedQty, accumulatedInventoryValue = CalculateAccumulativeValue(createInvoicePayload.Types, recordBefore, createInvoicePayload, 0)
	
		_, err := repository.InsertInvoice(createInvoicePayload, cogs, remainingQty, fifoInputStockMovementId, fifoInputPreAdjustmentRemainingQty, accumulatedQty, accumulatedInventoryValue)
		if err != nil {
			return "create record failed", err
		}

		// handle create clear phantom
		if (isPhantomExist) {
			var phantomQty int32
			if accumulatedQty > outStandingPhantom.UsedPhantomQty.Int32 {
				phantomQty = outStandingPhantom.UsedPhantomQty.Int32
			} else {
				phantomQty = outStandingPhantom.UsedPhantomQty.Int32 - accumulatedQty
			}
			phantomCogs := price * float64(phantomQty)
			phantomAccumulatedQty := accumulatedQty - phantomQty
			phantomAccumulatedInventoryValue := accumulatedInventoryValue - phantomCogs
			message, err = repository.InsertClearPhantomPurchase(createInvoicePayload.CreatedAt, phantomQty, phantomCogs, phantomAccumulatedQty, phantomAccumulatedInventoryValue)
			if err != nil {
				return message, err
			}
			usedPhantomQty := outStandingPhantom.UsedPhantomQty.Int32 - phantomQty
			message, err = repository.UpdateInvoiceUsedPhantomQty(usedPhantomQty, outStandingPhantom.Id)
			if err != nil {
				return message, err
			}
		}

	case common.Sales:
		// handle create new invoice
		firstInOutStandingBatch, _ := repository.GetFirstNonEmptyBatch()

		// using pointer to handle nullable value
		var firstFifoInputStockMovementId int32
		var firstFifoPreAdjusmentRemainingQty int32
		firstFifoInputStockMovementId = firstInOutStandingBatch.Id
		firstFifoPreAdjusmentRemainingQty = firstInOutStandingBatch.RemainingQty.Int32
		fifoInputStockMovementId = &firstFifoInputStockMovementId
		fifoInputPreAdjustmentRemainingQty = &firstFifoPreAdjusmentRemainingQty

		var totalCogsValue float64
		qtyLeft := createInvoicePayload.Qty
		for qtyLeft > 0 && (firstInOutStandingBatch != entity.Invoice{}) {
			if firstInOutStandingBatch.RemainingQty.Int32 > qtyLeft {
				totalCogsValue += firstInOutStandingBatch.Price.Float64 * (float64(qtyLeft))
				firstInOutStandingBatchRemainingQty := firstInOutStandingBatch.RemainingQty.Int32 - createInvoicePayload.Qty
				repository.UpdateInvoiceRemainingQty(firstInOutStandingBatchRemainingQty, firstInOutStandingBatch.Id)
				qtyLeft = 0
			} else {
				totalCogsValue += firstInOutStandingBatch.Price.Float64 * (float64(firstInOutStandingBatch.RemainingQty.Int32))
				qtyLeft = qtyLeft - firstInOutStandingBatch.RemainingQty.Int32
				repository.UpdateInvoiceRemainingQty(0, firstInOutStandingBatch.Id)
			}

			firstInOutStandingBatch, _ = repository.GetFirstNonEmptyBatch()
		}
		cogs = &totalCogsValue

		var phantomQty int32 = 0
		if (isNegative) {
			// create phantom invoice
			phantomQty = int32(math.Abs(float64(recordBefore.AccumulatedQty.Int32 - createInvoicePayload.Qty)))
			_, err = repository.InsertPhantomPurchase(createInvoicePayload.CreatedAt, createInvoicePayload.LocationId, phantomQty, createInvoicePayload.Qty, recordBefore.AccumulatedInventoryValue.Float64)
			if err != nil {
				return "create record failed", err
			}
			accumulatedQty = 0
			accumulatedInventoryValue = 0
		} else {
			accumulatedQty, accumulatedInventoryValue = CalculateAccumulativeValue(createInvoicePayload.Types, recordBefore, createInvoicePayload, totalCogsValue)
		}

		_, err = repository.InsertInvoice(createInvoicePayload, cogs, remainingQty, fifoInputStockMovementId, fifoInputPreAdjustmentRemainingQty, accumulatedQty, accumulatedInventoryValue)
		if err != nil {
			return "create record failed", err
		}
	
	case common.SalesReturn:
		// handle create new record
		batchBeingReturnedTo, err := repository.GetInvoiceById(createInvoicePayload.SalesReturnId.Int32)
		if err != nil {
			return "create record failed", err
		}
		remainingQty = createInvoicePayload.Qty
		accumulatedQty, accumulatedInventoryValue = CalculateAccumulativeValue(createInvoicePayload.Types, recordBefore, createInvoicePayload, 0)
		price = batchBeingReturnedTo.Cogs.Float64 / float64(batchBeingReturnedTo.Qty.Int32)
	
		_, err = repository.InsertInvoice(createInvoicePayload, cogs, remainingQty, fifoInputStockMovementId, fifoInputPreAdjustmentRemainingQty, accumulatedQty, accumulatedInventoryValue)
		if err != nil {
			return "create record failed", err
		}

		if (isPhantomExist) {
			// handle create clear phantom
			var phantomQty int32
			if accumulatedQty > outStandingPhantom.UsedPhantomQty.Int32 {
				phantomQty = outStandingPhantom.UsedPhantomQty.Int32
			} else {
				phantomQty = outStandingPhantom.UsedPhantomQty.Int32 - accumulatedQty
			}
			phantomCogs := price * float64(phantomQty)
			phantomAccumulatedQty := accumulatedQty - phantomQty
			phantomAccumulatedInventoryValue := accumulatedInventoryValue - phantomCogs
			message, err = repository.InsertClearPhantomPurchase(createInvoicePayload.CreatedAt, phantomQty, phantomCogs, phantomAccumulatedQty, phantomAccumulatedInventoryValue)
			if err != nil {
				return message, err
			}
			usedPhantomQty := outStandingPhantom.UsedPhantomQty.Int32 - phantomQty
			message, err = repository.UpdateInvoiceUsedPhantomQty(usedPhantomQty, outStandingPhantom.Id)
			if err != nil {
				return message, err
			}
		}

	case common.PurchaseReturn:
		// handle create new invoice
		firstInOutStandingBatch, _ := repository.GetFirstNonEmptyBatch()

		// using pointer to handle nullable value
		var firstFifoInputStockMovementId int32
		var firstFifoPreAdjusmentRemainingQty int32
		firstFifoInputStockMovementId = firstInOutStandingBatch.Id
		firstFifoPreAdjusmentRemainingQty = firstInOutStandingBatch.RemainingQty.Int32
		fifoInputStockMovementId = &firstFifoInputStockMovementId
		fifoInputPreAdjustmentRemainingQty = &firstFifoPreAdjusmentRemainingQty

		var totalCogsValue float64
		qtyLeft := createInvoicePayload.Qty
		for qtyLeft > 0 && (firstInOutStandingBatch != entity.Invoice{}) {
			if firstInOutStandingBatch.RemainingQty.Int32 > qtyLeft {
				totalCogsValue += firstInOutStandingBatch.Price.Float64 * (float64(qtyLeft))
				firstInOutStandingBatchRemainingQty := firstInOutStandingBatch.RemainingQty.Int32 - createInvoicePayload.Qty
				repository.UpdateInvoiceRemainingQty(firstInOutStandingBatchRemainingQty, firstInOutStandingBatch.Id)
				qtyLeft = 0
			} else {
				totalCogsValue += firstInOutStandingBatch.Price.Float64 * (float64(firstInOutStandingBatch.RemainingQty.Int32))
				qtyLeft = qtyLeft - firstInOutStandingBatch.RemainingQty.Int32
				repository.UpdateInvoiceRemainingQty(0, firstInOutStandingBatch.Id)
			}

			firstInOutStandingBatch, _ = repository.GetFirstNonEmptyBatch()
		}
		cogs = &totalCogsValue
		createInvoicePayload.Price = totalCogsValue / float64(createInvoicePayload.Qty)

		var phantomQty int32 = 0
		if (isNegative) {
			// create phantom invoice
			phantomQty = int32(math.Abs(float64(recordBefore.AccumulatedQty.Int32 - createInvoicePayload.Qty)))
			_, err = repository.InsertPhantomPurchase(createInvoicePayload.CreatedAt, createInvoicePayload.LocationId, phantomQty, createInvoicePayload.Qty, recordBefore.AccumulatedInventoryValue.Float64)
			if err != nil {
				return "create record failed", err
			}
			accumulatedQty = 0
			accumulatedInventoryValue = 0
		} else {
			accumulatedQty, accumulatedInventoryValue = CalculateAccumulativeValue(createInvoicePayload.Types, recordBefore, createInvoicePayload, totalCogsValue)
		}

		_, err = repository.InsertInvoice(createInvoicePayload, cogs, remainingQty, fifoInputStockMovementId, fifoInputPreAdjustmentRemainingQty, accumulatedQty, accumulatedInventoryValue)
		if err != nil {
			return "create record failed", err
		}
	}

	if (isBackdate) {
		/*
			get newer non phantom output record -> update remaining_qty of the record
			reset remaining_qty of the record after that
			handle reset remaining qty
				get last output record after the backdated record
					get last updated batch using output.fifo_stock_movement_id reset it to output.fifo_input_pre_adjustment_remaining_qty
					reset the rest of input record remaining_qty to its qty
		*/

		// handle backdate
		_, err = repository.DeleteNewerPhantomInvoice(createInvoicePayload.CreatedAt)
		if err != nil {
			return "delete newer phantom record failed", err
		}

		newerNonPhantomOutputRecord, err := repository.GetNewerNonPhantomOutputInvoice(createInvoicePayload.CreatedAt)
		if err != nil {
			return "get newer non phantom output record failed", err
		}

		_, err = repository.UpdateInvoiceRemainingQty(newerNonPhantomOutputRecord.FifoInputPreAdjustmentRemainingQty.Int32, newerNonPhantomOutputRecord.FifoInputStockMovementId.Int32)
		if err != nil {
			return "update remaining qty last output record failed", err
		}

		_, err = repository.ResetRemainingQty(newerNonPhantomOutputRecord.CreatedAt, newerNonPhantomOutputRecord.Id)
		if err != nil {
			return "reset remaining qty failed", err
		}

		for _, record := range recordAfter {
			recordBefore = repository.GetLatestUniqueInvoice(record.CreatedAt, record.Id)

			outStandingPhantom := repository.GetOutstandingPhantomPurchase(record.CreatedAt)

			stockDocumentType = record.StockDocumentType
			isNegative = recordBefore.AccumulatedQty.Int32 < record.Qty.Int32
			isPhantomExist = outStandingPhantom != entity.Invoice{}

			switch strings.ToLower(stockDocumentType) {
			case common.Purchase:
				// handle create new record
				price = record.Price.Float64
				remainingQty = record.Qty.Int32
				accumulatedQty, accumulatedInventoryValue = CalculateBackdateAccumulativeValue(record.Types, recordBefore, record, 0)
				
				_, err := repository.UpdateInvoice(cogs, remainingQty, nil, nil, accumulatedQty, accumulatedInventoryValue, record.Id)
				if err != nil {
					return "update record failed", err
				}

				if (isPhantomExist) {
					// handle create clear phantom
					var phantomQty int32
					if accumulatedQty > outStandingPhantom.UsedPhantomQty.Int32 {
						phantomQty = outStandingPhantom.UsedPhantomQty.Int32
					} else {
						phantomQty = outStandingPhantom.UsedPhantomQty.Int32 - accumulatedQty
					}
					phantomCogs := price * float64(phantomQty)
					phantomAccumulatedQty := accumulatedQty - phantomQty
					phantomAccumulatedInventoryValue := accumulatedInventoryValue - phantomCogs
					_, err = repository.InsertClearPhantomPurchase(record.CreatedAt, phantomQty, phantomCogs, phantomAccumulatedQty, phantomAccumulatedInventoryValue)
					if err != nil {
						return "create clear phantom record failed", err
					}
					usedPhantomQty := outStandingPhantom.UsedPhantomQty.Int32 - phantomQty
					message, err = repository.UpdateInvoiceUsedPhantomQty(usedPhantomQty, outStandingPhantom.Id)
					if err != nil {
						return message, err
					}
				}

			case common.Sales:
				// handle create new invoice
				firstInOutStandingBatch, _ := repository.GetFirstNonEmptyBatch()

				// using pointer to handle nullable value
				var firstFifoInputStockMovementId int32
				var firstFifoPreAdjusmentRemainingQty int32
				firstFifoInputStockMovementId = firstInOutStandingBatch.Id
				firstFifoPreAdjusmentRemainingQty = firstInOutStandingBatch.RemainingQty.Int32
				fifoInputStockMovementId = &firstFifoInputStockMovementId
				fifoInputPreAdjustmentRemainingQty = &firstFifoPreAdjusmentRemainingQty

				var totalCogsValue float64
				qtyLeft := record.Qty.Int32
				for qtyLeft > 0 && (firstInOutStandingBatch != entity.Invoice{}) {
					if firstInOutStandingBatch.RemainingQty.Int32 > qtyLeft {
						totalCogsValue += firstInOutStandingBatch.Price.Float64 * (float64(qtyLeft))
						firstInOutStandingBatchRemainingQty := firstInOutStandingBatch.RemainingQty.Int32 - record.Qty.Int32
						repository.UpdateInvoiceRemainingQty(firstInOutStandingBatchRemainingQty, firstInOutStandingBatch.Id)
						qtyLeft = 0
					} else {
						totalCogsValue += firstInOutStandingBatch.Price.Float64 * (float64(firstInOutStandingBatch.RemainingQty.Int32))
						qtyLeft = qtyLeft - firstInOutStandingBatch.RemainingQty.Int32
						repository.UpdateInvoiceRemainingQty(0, firstInOutStandingBatch.Id)
					}

					firstInOutStandingBatch, _ = repository.GetFirstNonEmptyBatch()
				}
				cogs = &totalCogsValue

				var phantomQty int32 = 0
				if (isNegative) {
					// create phantom invoice
					phantomQty = int32(math.Abs(float64(recordBefore.AccumulatedQty.Int32 - record.Qty.Int32)))
					_, err = repository.InsertPhantomPurchase(record.CreatedAt, record.LocationId, phantomQty, record.Qty.Int32, recordBefore.AccumulatedInventoryValue.Float64)
					if err != nil {
						return "create record failed", err
					}
					accumulatedQty = 0
					accumulatedInventoryValue = 0
				} else {
					accumulatedQty, accumulatedInventoryValue = CalculateBackdateAccumulativeValue(record.Types, recordBefore, record, totalCogsValue)
				}

				_, err := repository.UpdateInvoice(cogs, 0, fifoInputStockMovementId, fifoInputPreAdjustmentRemainingQty, accumulatedQty, accumulatedInventoryValue, record.Id)
				if err != nil {
					return "update record failed", err
				}
			
			case common.SalesReturn:
				// handle create new record
				batchBeingReturnedTo, err := repository.GetInvoiceById(record.SalesReturnId.Int32)
				if err != nil {
					return "create record failed", err
				}
				remainingQty = record.Qty.Int32
				accumulatedQty, accumulatedInventoryValue = CalculateBackdateAccumulativeValue(record.Types, recordBefore, record, 0)
				price = batchBeingReturnedTo.Cogs.Float64 / float64(batchBeingReturnedTo.Qty.Int32)
			
				_, err = repository.UpdateInvoice(cogs, remainingQty, nil, nil, accumulatedQty, accumulatedInventoryValue, record.Id)
				if err != nil {
					return "update record failed", err
				}

				if (isPhantomExist) {
					// handle create clear phantom
					var phantomQty int32
					if accumulatedQty > outStandingPhantom.UsedPhantomQty.Int32 {
						phantomQty = outStandingPhantom.UsedPhantomQty.Int32
					} else {
						phantomQty = outStandingPhantom.UsedPhantomQty.Int32 - accumulatedQty
					}
					phantomCogs := price * float64(phantomQty)
					phantomAccumulatedQty := accumulatedQty - phantomQty
					phantomAccumulatedInventoryValue := accumulatedInventoryValue - phantomCogs
					_, err = repository.InsertClearPhantomPurchase(record.CreatedAt, phantomQty, phantomCogs, phantomAccumulatedQty, phantomAccumulatedInventoryValue)
					if err != nil {
						return "create record failed", err
					}
					usedPhantomQty := outStandingPhantom.UsedPhantomQty.Int32 - phantomQty
					message, err = repository.UpdateInvoiceUsedPhantomQty(usedPhantomQty, outStandingPhantom.Id)
					if err != nil {
						return message, err
					}
				}

			case common.PurchaseReturn:
				// handle create new invoice
				firstInOutStandingBatch, _ := repository.GetFirstNonEmptyBatch()

				// using pointer to handle nullable value
				var firstFifoInputStockMovementId int32
				var firstFifoPreAdjusmentRemainingQty int32
				firstFifoInputStockMovementId = firstInOutStandingBatch.Id
				firstFifoPreAdjusmentRemainingQty = firstInOutStandingBatch.RemainingQty.Int32
				fifoInputStockMovementId = &firstFifoInputStockMovementId
				fifoInputPreAdjustmentRemainingQty = &firstFifoPreAdjusmentRemainingQty

				var totalCogsValue float64
				qtyLeft := record.Qty.Int32
				for qtyLeft > 0 && (firstInOutStandingBatch != entity.Invoice{}) {
					if firstInOutStandingBatch.RemainingQty.Int32 > qtyLeft {
						totalCogsValue += firstInOutStandingBatch.Price.Float64 * (float64(qtyLeft))
						firstInOutStandingBatchRemainingQty := firstInOutStandingBatch.RemainingQty.Int32 - record.Qty.Int32
						repository.UpdateInvoiceRemainingQty(firstInOutStandingBatchRemainingQty, firstInOutStandingBatch.Id)
						qtyLeft = 0
					} else {
						totalCogsValue += firstInOutStandingBatch.Price.Float64 * (float64(firstInOutStandingBatch.RemainingQty.Int32))
						qtyLeft = qtyLeft - firstInOutStandingBatch.RemainingQty.Int32
						repository.UpdateInvoiceRemainingQty(0, firstInOutStandingBatch.Id)
					}

					firstInOutStandingBatch, _ = repository.GetFirstNonEmptyBatch()
				}
				cogs = &totalCogsValue
				record.Price.Float64 = totalCogsValue / float64(record.Qty.Int32)

				var phantomQty int32 = 0
				if (isNegative) {
					// create phantom invoice
					phantomQty = int32(math.Abs(float64(recordBefore.AccumulatedQty.Int32 - record.Qty.Int32)))
					_, err = repository.InsertPhantomPurchase(record.CreatedAt, record.LocationId, phantomQty, record.Qty.Int32, recordBefore.AccumulatedInventoryValue.Float64)
					if err != nil {
						return "create record failed", err
					}
					accumulatedQty = 0
					accumulatedInventoryValue = 0
				} else {
					accumulatedQty, accumulatedInventoryValue = CalculateBackdateAccumulativeValue(record.Types, recordBefore, record, totalCogsValue)
				}

				_, err := repository.UpdateInvoice(cogs, 0, fifoInputStockMovementId, fifoInputPreAdjustmentRemainingQty, accumulatedQty, accumulatedInventoryValue, record.Id)
				if err != nil {
					return "update record failed", err
				}
			}
		}
	}

	return "create invoice success", nil
}

func CalculateAccumulativeValue(types string, recordBefore entity.Invoice, createInvoicePayload dto.CreateInvoiceDto, cogs float64) (accumulatedQty int32, accumulatedInventoryValue float64) {
	 if types == common.TypesOutput {
		accumulatedQty = recordBefore.AccumulatedQty.Int32 - createInvoicePayload.Qty
		accumulatedInventoryValue = recordBefore.AccumulatedInventoryValue.Float64 - cogs
	 } else {
		accumulatedQty = recordBefore.AccumulatedQty.Int32 + createInvoicePayload.Qty
		accumulatedInventoryValue = recordBefore.AccumulatedInventoryValue.Float64 + (float64(createInvoicePayload.Qty) * createInvoicePayload.Price)
	 }

	 return accumulatedQty, accumulatedInventoryValue
}

func CalculateBackdateAccumulativeValue(types string, recordBefore entity.Invoice, record entity.Invoice, cogs float64) (accumulatedQty int32, accumulatedInventoryValue float64) {
	 if types == common.TypesOutput {
		accumulatedQty = recordBefore.AccumulatedQty.Int32 - record.Qty.Int32
		accumulatedInventoryValue = recordBefore.AccumulatedInventoryValue.Float64 - cogs
	 } else {
		accumulatedQty = recordBefore.AccumulatedQty.Int32 + record.Qty.Int32
		accumulatedInventoryValue = recordBefore.AccumulatedInventoryValue.Float64 + (float64(record.Qty.Int32) * record.Price.Float64)
	 }

	 return accumulatedQty, accumulatedInventoryValue
}

