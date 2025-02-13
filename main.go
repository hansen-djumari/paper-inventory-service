package main

import (
	"fmt"
	"paper/inventory-api/controller"
	"paper/inventory-api/db"

	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.New()
	
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Running",
		})
	})
	invoiceRoute := r.Group("/invoice")
	{
		// invoiceRoute.GET("/", controller.GetInvoicesByCreateDate)
		invoiceRoute.POST("/", controller.CreateInvoice)
	}

	db.ConnectDatabase()
	r.Run()

	fmt.Println("running")
}