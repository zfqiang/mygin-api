package test

import (
	"fmt"
	"mygin-api/internal/model/genm"

	"github.com/bwmarrin/snowflake"
	"gorm.io/gorm"
)

type GenData struct {
	db *gorm.DB
}

func (g *GenData) GenOrderData(count int) {
	orders := make([]genm.Order, count)
	for i := 0; i < count; i++ {
		node, _ := snowflake.NewNode(10)
		orders[i] = genm.Order{
			ID:      int64(i + 1),
			OrderNo: fmt.Sprintf("ORD%s", node.Generate().String),
		}
	}
	g.db.Create(&orders)
}
