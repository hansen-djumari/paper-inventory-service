package controller

import (
	"paper/inventory-api/common/dto"
	"paper/inventory-api/common/types"
	"paper/inventory-api/service"

	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

func CreateInvoice(c *gin.Context) {
	body := dto.CreateInvoiceDto{}

	data, err := c.GetRawData()
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, types.ApiResponse{Status: http.StatusInternalServerError, Message: "create invoice failed", Errors: err, Data: nil})
		return
	}
	err = json.Unmarshal(data, &body)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, types.ApiResponse{Status: http.StatusInternalServerError, Errors: err, Message: "create invoice failed", Data: nil})
		return
	}

	message, err := service.CreateRecord(body)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, types.ApiResponse{Status: http.StatusInternalServerError, Errors: err, Message: message, Data: nil})
		return
	}

	c.JSON(http.StatusOK, types.ApiResponse{Status: http.StatusOK, Errors: nil, Message: message, Data: nil})
}