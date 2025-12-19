package main

import (
	"context"
	"log"
	"net/http"

	"connectrpc.com/connect"
	"github.com/pdcgo/schema/services/common/v1"
	withdrawal_iface_v1 "github.com/pdcgo/schema/services/withdrawal_iface/v1"
	withdrawal_ifaceconnect_v1 "github.com/pdcgo/schema/services/withdrawal_iface/v1/withdrawal_ifaceconnect"
	"github.com/pdcgo/schema/services/withdrawal_iface/v2"
	"github.com/pdcgo/schema/services/withdrawal_iface/v2/withdrawal_ifaceconnect"
)

func main() {

	ctx := context.Background()

	// cfg, err := configs.NewProductionConfig()
	// if err != nil {
	// 	panic(err)
	// }
	endpoint := "http://localhost:8082"
	token := "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJVc2VySUQiOjQxLCJTdXBlclVzZXIiOmZhbHNlLCJWYWxpZFVudGlsIjoxNzU4NDQ1MTAzMDcyNzAxLCJGcm9tIjoic2VsbGluZyIsIlVzZXJBZ2VudCI6IiIsIkNyZWF0ZWRBdCI6MTc1ODM1ODcwMzA3MjcwMX0.-G7OrdLgZe15WfC2VvLfistKmN37iYZIdo7TlXOmC9M"

	log.Println(withdrawal_ifaceconnect_v1.WithdrawalServiceName)
	// clientv1 := withdrawal_ifaceconnect_v1.NewWithdrawalServiceClient(
	// 	http.DefaultClient,
	// 	endpoint,
	// 	connect.WithGRPC(),
	// )

	// _, err := clientv1.Run(ctx, &connect.Request[withdrawal_iface_v1.RunRequest]{
	// 	Msg: &withdrawal_iface_v1.RunRequest{},
	// })
	// log.Println(err)

	client := withdrawal_ifaceconnect.NewWithdrawalServiceClient(
		http.DefaultClient,
		endpoint,
		connect.WithGRPC(),
	)

	req := connect.NewRequest(&withdrawal_iface.SubmitWithdrawalRequest{
		TeamId: 31,
		MpSubmit: &withdrawal_iface.MpSubmit{
			MpId:   619,
			MpType: common.MarketplaceType_MARKETPLACE_TYPE_SHOPEE,
		},
		Source:      withdrawal_iface_v1.ImporterSource_IMPORTER_SOURCE_XLS,
		ResourceUri: "withdrawal_resources/13_yumona.id_08_26_2025_09_20.xlsx",
	})

	req.Header().Set("Authorization", token)

	stream, err := client.SubmitWithdrawal(ctx, req)

	if err != nil {
		panic(err)
	}

	for stream.Receive() {
		msg := stream.Msg()
		log.Println(msg)
	}

	err = stream.Err()
	if err != nil {
		panic(err)
	}

}
