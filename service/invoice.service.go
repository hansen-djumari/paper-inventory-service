package service

import (
	"math"
	"paper/inventory-api/common"
	"paper/inventory-api/common/dto"
	"paper/inventory-api/entity"
	"paper/inventory-api/repository"
	"strings"
)

func CreateRecord(createInvoicePayload dto.CreateInvoiceDto) (message string, err error) {
	recordAfter, err := repository.GetInvoicesByCreatedAt(createInvoicePayload.CreatedAt, ">", 1, 0)
	if err != nil {
		return "create record failed", err
	}
	recordBefore := repository.GetLatestInvoice(createInvoicePayload.CreatedAt)

	outStandingPhantom := repository.GetOutstandingPhantomPurchase()

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
		if (recordBefore != entity.Invoice{}) {
			accumulatedQty = recordBefore.AccumulatedQty.Int32 + createInvoicePayload.Qty
			accumulatedInventoryValue = recordBefore.AccumulatedInventoryValue.Float64 + (float64(createInvoicePayload.Qty) * createInvoicePayload.Price)
		} else {
			accumulatedQty = createInvoicePayload.Qty
			accumulatedInventoryValue = float64(createInvoicePayload.Qty) * createInvoicePayload.Price
		}
		_, err := repository.InsertInvoice(createInvoicePayload, cogs, remainingQty, fifoInputStockMovementId, fifoInputPreAdjustmentRemainingQty, accumulatedQty, accumulatedInventoryValue)
		if err != nil {
			return "create record failed", err
		}

		if (isPhantomExist) {
			// handle create clear phantom
			var phantomQty int32
			if accumulatedQty > outStandingPhantom.RemainingQty.Int32 {
				phantomQty = outStandingPhantom.RemainingQty.Int32
			} else {
				phantomQty = outStandingPhantom.RemainingQty.Int32 - accumulatedQty
			}
			phantomCogs := price * float64(phantomQty)
			phantomAccumulatedQty := accumulatedQty - phantomQty
			phantomAccumulatedInventoryValue := accumulatedInventoryValue - phantomCogs
			_, err = repository.InsertClearPhantomPurchase(createInvoicePayload.CreatedAt, phantomQty, phantomCogs, phantomAccumulatedQty, phantomAccumulatedInventoryValue)
			if err != nil {
				return "create record failed", err
			}
		}

		if (isBackdate) {
			/*
				handle reset remaining qty
					get last output record after the backdated record
						get last updated batch using output.fifo_stock_movement_id reset it to output.fifo_input_pre_adjustment_remaining_qty
						reset the rest of input record remaining_qty to its qty
			*/
			lastOutputRecord, err := repository.GetLastOutputInvoice(createInvoicePayload.CreatedAt)
			if err != nil {
				return "create record failed", err
			}
			_, err = repository.ResetRemainingQty(lastOutputRecord.CreatedAt)
			if err != nil {
				return "create record failed", err
			}

			// handle backdate
			_, err = repository.DeleteNewerPhantomInvoice(createInvoicePayload.CreatedAt)
			if err != nil {
				return "create record failed", err
			}

			// for _, record := range recordAfter {
			// 	price = createInvoicePayload.Price
			// 	remainingQty = createInvoicePayload.Qty
			// 	accumulatedQty = recordBefore[0].AccumulatedQty.Int32 + createInvoicePayload.Qty
			// 	accumulatedInventoryValue = recordBefore[0].AccumulatedInventoryValue.Float64 + (float64(createInvoicePayload.Qty) * createInvoicePayload.Price)
			// 	_, err := repository.UpdateInvoice(cogs, remainingQty, fifoInputStockMovementId, accumulatedQty, accumulatedInventoryValue, record.Id)
			// 	if err != nil {
			// 		return "create record failed", err
			// 	}
			// }
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
			if (recordBefore != entity.Invoice{}) {
				accumulatedQty = recordBefore.AccumulatedQty.Int32 - createInvoicePayload.Qty
				accumulatedInventoryValue = recordBefore.AccumulatedInventoryValue.Float64 - totalCogsValue
			} else {
				accumulatedQty = createInvoicePayload.Qty
				accumulatedInventoryValue = float64(createInvoicePayload.Qty) * createInvoicePayload.Price
			}
		}

		_, err = repository.InsertInvoice(createInvoicePayload, cogs, remainingQty, fifoInputStockMovementId, fifoInputPreAdjustmentRemainingQty, accumulatedQty, accumulatedInventoryValue)
		if err != nil {
			return "create record failed", err
		}
	}

	return "create invoice success", nil
}
